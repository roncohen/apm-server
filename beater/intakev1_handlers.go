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
	"net/http"
	"regexp"
	"strings"

	"github.com/elastic/apm-server/decoder"
	"github.com/elastic/apm-server/model"
	err "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/model/metric"
	"github.com/elastic/apm-server/model/sourcemap"
	"github.com/elastic/apm-server/model/transaction"
	"github.com/elastic/apm-server/validation"

	"github.com/elastic/beats/libbeat/logp"
	"github.com/santhosh-tekuri/jsonschema"
)

type PayloadDecoder func(map[string]interface{}) (*model.Metadata, []model.Transformable, error)

type ReqMetadataAugmenter func(config *Config) func(*http.Request) map[string]interface{}

type ConfigurableHandler func(*Config, http.Handler) http.Handler

func (v v1Route) handler(beaterConfig *Config, report reporter) func(*http.Request) serverResponse {
	reqDecoder := v.reqDecoder(
		beaterConfig,
		decoder.DecodeLimitJSONData(beaterConfig.MaxUnzippedSize),
	)

	var transformConfig model.TransformConfig
	if v.v1RouteType.tranformConfig != nil {
		transformConfig = v.v1RouteType.tranformConfig(beaterConfig)
	}

	return func(r *http.Request) serverResponse {
		if r.Method != "POST" {
			return methodNotAllowedResponse
		}

		data, err := reqDecoder(r)
		if err != nil {
			if strings.Contains(err.Error(), "request body too large") {
				return requestTooLargeResponse
			}
			return cannotDecodeResponse(err)

		}

		if err = validation.Validate(data, v.V1PayloadType.Schema); err != nil {
			return cannotValidateResponse(err)
		}

		metadata, payload, err := v.V1PayloadType.PayloadDecoder(data)
		if err != nil {
			return cannotDecodeResponse(err)
		}

		tctx := &model.TransformContext{
			Config:   transformConfig,
			Metadata: *metadata,
		}

		preq := pendingReq{
			payload:          payload,
			transformContext: tctx,
		}

		if err = report(r.Context(), preq); err != nil {
			if strings.Contains(err.Error(), "publisher is being stopped") {
				return serverShuttingDownResponse(err)
			}
			return fullQueueResponse(err)
		}

		return acceptedResponse
	}
}

func (v v1Route) Handler(beaterConfig *Config, report reporter) http.Handler {
	internalHandler := v.handler(beaterConfig, report)

	return v.routeTypeHandler(beaterConfig,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			sendStatus(w, r, internalHandler(r))
		}))
}

func sourcemappingConfig(beaterConfig *Config) model.TransformConfig {
	smapper, err := beaterConfig.Frontend.memoizedSmapMapper()
	if err != nil {
		logp.NewLogger("handler").Error(err.Error())
	}
	return model.TransformConfig{
		SmapMapper:          smapper,
		LibraryPattern:      regexp.MustCompile(beaterConfig.Frontend.LibraryPattern),
		ExcludeFromGrouping: regexp.MustCompile(beaterConfig.Frontend.ExcludeFromGrouping),
	}
}

// V1PayloadType specifies the jsonschema, payload decoding and metrics
// for a specific v1 payload for a model
type V1PayloadType struct {
	Name           string
	Schema         *jsonschema.Schema
	PayloadDecoder PayloadDecoder
}

func (v *V1PayloadType) Validate(raw map[string]interface{}) error {
	return validation.Validate(raw, v.Schema)
}

type v1RouteType struct {
	name             string
	routeTypeHandler ConfigurableHandler
	reqDecoder       func(*Config, decoder.ReqDecoder) decoder.ReqDecoder
	tranformConfig   func(*Config) model.TransformConfig
}

var (
	// V1 style payload handling for a model
	// Each payload type can be paired with a different route type, see below
	TransactionV1Route = V1PayloadType{
		"transaction",
		transaction.PayloadSchema(),
		transaction.DecodePayload,
	}

	ErrorV1Route = V1PayloadType{
		"error",
		err.PayloadSchema(),
		err.DecodePayload,
	}

	MetricV1Route = V1PayloadType{
		"metric",
		metric.PayloadSchema(),
		metric.DecodePayload,
	}

	SourcemapV1Route = V1PayloadType{
		"sourcemap",
		sourcemap.PayloadSchema(),
		sourcemap.DecodePayload,
	}
)

type v1Route struct {
	V1PayloadType
	v1RouteType
}

var V1Routes = map[string]v1Route{
	BackendTransactionsURL: {
		TransactionV1Route,
		BackendRouteType,
	},
	FrontendTransactionsURL: {
		TransactionV1Route,
		FrontendRouteType,
	},
	MetricsURL: {
		MetricV1Route,
		MetricsRouteType,
	},
	BackendErrorsURL: {
		ErrorV1Route,
		BackendRouteType,
	},
	FrontendErrorsURL: {
		ErrorV1Route,
		FrontendRouteType,
	},
	SourcemapsURL: {
		SourcemapV1Route,
		SourcemapRouteType,
	},
}
