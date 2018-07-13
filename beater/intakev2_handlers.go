// Licensed to Elasticsearch B.V. under one or more contributor
// license agreements. See the NOTICE file distributed with
// this work for additional information regarding copyright
// ownership. Elasticsearch B.V. licenses this file to you under
// the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.

package beater

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"

	"github.com/elastic/apm-server/model/metric"
	"github.com/pkg/errors"

	"github.com/elastic/apm-server/validation"
	"github.com/santhosh-tekuri/jsonschema"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/span"
	"github.com/elastic/apm-server/model/transaction"
)

type v2Route struct {
	v1RouteType
}

type NDJSONStreamReader struct {
	stream *bufio.Reader
}

const batchSize = 20

func (sr *NDJSONStreamReader) Read() (map[string]interface{}, error) {
	buf, readErr := sr.stream.ReadBytes('\n')
	if readErr != nil && readErr != io.EOF {
		return nil, readErr
	}

	tmpreader := ioutil.NopCloser(bytes.NewBuffer(buf))
	decoded, err := decoder.DecodeJSONData(tmpreader)
	if err != nil {
		return nil, err
	}
	return decoded, readErr // this might be io.EOF
}

func StreamDecodeLimitJSONData(req *http.Request, maxSize int64) (*NDJSONStreamReader, error) {
	contentType := req.Header.Get("Content-Type")
	if !strings.Contains(contentType, "application/ndjson") {
		return nil, fmt.Errorf("invalid content type: %s", req.Header.Get("Content-Type"))
	}

	reader, err := decoder.CompressedRequestReader(maxSize)(req)
	if err != nil {
		return nil, err
	}

	return &NDJSONStreamReader{bufio.NewReader(reader)}, nil
}

var Models = []struct {
	key          string
	schema       *jsonschema.Schema
	modelDecoder func(interface{}, error) (model.Transformable, error)
}{
	{
		"transaction",
		transaction.ModelSchema(),
		transaction.DecodeEvent,
	},
	{
		"span",
		span.ModelSchema(),
		span.DecodeSpan,
	},
	{
		"metric",
		metric.ModelSchema(),
		metric.DecodeMetric,
	},
}

// handleRawModel validates and decodes a single json object into its struct form
func (v v2Route) handleRawModel(rawModel map[string]interface{}) (model.Transformable, serverResponse) {
	for _, model := range Models {
		if entry, ok := rawModel[model.key]; ok {
			err := validation.Validate(entry, model.schema)
			if err != nil {
				return nil, cannotValidateResponse(err)
			}

			tr, err := model.modelDecoder(entry, err)
			if err != nil {
				return tr, cannotDecodeResponse(err)
			}
			return tr, serverResponse{}
		}
	}
	return nil, cannotValidateResponse(errors.New("did not recognize object type"))
}

// readBatch will read up to `batchSize` objects from the ndjson stream
// it returns a slice of transformables, a serverResponse and a bool that indicates if we're at EOF.
func (v v2Route) readBatch(batchSize int, reader *NDJSONStreamReader) ([]model.Transformable, serverResponse, bool) {
	var err error
	var rawModel map[string]interface{}

	transformables := []model.Transformable{}
	for i := 0; i < batchSize && err == nil; i++ {
		rawModel, err = reader.Read()
		if err != nil && err != io.EOF {
			return nil, cannotDecodeResponse(err), false
		}

		if rawModel != nil {
			tr, resp := v.handleRawModel(rawModel)
			if resp.IsError() {
				return nil, resp, false
			}
			transformables = append(transformables, tr)
		}
	}

	return transformables, serverResponse{}, err == io.EOF
}

func (v v2Route) readMetadata(r *http.Request, beaterConfig *Config, ndjsonReader *NDJSONStreamReader) (*model.Metadata, serverResponse) {
	// first item is the metadata object
	rawData, err := ndjsonReader.Read()
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}

	rawMetadata, ok := rawData["metadata"].(map[string]interface{})
	if !ok {
		return nil, cannotValidateResponse(errors.New("invalid metadata format"))
	}

	// augment the metadata object with information from the request, like user-agent or remote address
	metadataDecoder := func(*http.Request) (map[string]interface{}, error) { return rawMetadata, nil }
	rawMetadata, err = v.reqDecoder(beaterConfig, metadataDecoder)(r)
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}

	// validate the metadata object against our jsonschema
	err = validation.Validate(rawMetadata, model.MetadataSchema())
	if err != nil {
		return nil, cannotValidateResponse(err)
	}

	// create a metadata struct
	metadata, err := model.DecodeMetadata(rawMetadata, err)
	if err != nil {
		return nil, cannotDecodeResponse(err)
	}
	return metadata, serverResponse{}
}

func (v v2Route) handler(r *http.Request, beaterConfig *Config, report reporter) serverResponse {
	ndjsonReader, err := StreamDecodeLimitJSONData(r, beaterConfig.MaxUnzippedSize)
	if err != nil {
		return cannotDecodeResponse(err)
	}

	metadata, serverResponse := v.readMetadata(r, beaterConfig, ndjsonReader)
	if serverResponse.IsError() {
		return serverResponse
	}

	var tcfg model.TransformConfig
	if v.tranformConfig != nil {
		tcfg = v.tranformConfig(beaterConfig)
	}

	tctx := &model.TransformContext{
		Config:   tcfg,
		Metadata: *metadata,
	}

	for {
		transformables, serverResponse, eof := v.readBatch(batchSize, ndjsonReader)
		if transformables != nil {
			report(r.Context(), pendingReq{
				payload:          transformables,
				transformContext: tctx,
			})
		}

		if serverResponse.IsError() {
			return serverResponse
		}

		if eof {
			break
		}
	}
	return acceptedResponse
}

func (v v2Route) Handler(beaterConfig *Config, report reporter) http.Handler {
	return v.routeTypeHandler(beaterConfig, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sendStatus(w, r, v.handler(r, beaterConfig, report))
	}))
}

var V2Routes = map[string]v2Route{
	V2BackendURL:  v2Route{BackendRouteType},
	V2FrontendURL: v2Route{FrontendRouteType},
}
