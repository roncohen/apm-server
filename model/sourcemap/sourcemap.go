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

package sourcemap

import (
	"time"

	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/sourcemap/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"

	smap "github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/santhosh-tekuri/jsonschema"
)

var cachedPayloadSchema = validation.CreateSchema(schema.PayloadSchema, "sourcemap")

func PayloadSchema() *jsonschema.Schema {
	return cachedPayloadSchema
}

type Payload struct {
	ServiceName    string
	ServiceVersion string
	Sourcemap      string
	BundleFilepath string
}

var processorEntry = common.MapStr{"name": "sourcemap", "event": "sourcemap"}

func (pa *Payload) Events(tctx *model.TransformContext) []beat.Event {
	if pa == nil {
		return nil
	}

	conf := tctx.Config
	if conf.SmapMapper == nil {
		logp.NewLogger("sourcemap").Error("Sourcemap Accessor is nil, cache cannot be invalidated.")
	} else {
		conf.SmapMapper.NewSourcemapAdded(smap.Id{
			ServiceName:    pa.ServiceName,
			ServiceVersion: pa.ServiceVersion,
			Path:           pa.BundleFilepath,
		})
	}

	ev := beat.Event{
		Fields: common.MapStr{
			"processor": processorEntry,
			"sourcemap": common.MapStr{
				"bundle_filepath": pa.BundleFilepath,
				"service":         common.MapStr{"name": pa.ServiceName, "version": pa.ServiceVersion},
				"sourcemap":       pa.Sourcemap,
			},
		},
		Timestamp: time.Now(),
	}
	return []beat.Event{ev}
}

// func DecodePayload(raw map[string]interface{}) (*model.Metadata, []*Payload, error) {
// 	if raw == nil {
// 		return nil, nil, nil
// 	}

// 	var err error
// 	var metadata model.Metadata
// 	metadata, err = model.DecodeMetadata(raw, err)

// 	decoder := utility.ManualDecoder{}
// 	txs := decoder.InterfaceArr(raw, "transactions")
// 	err = decoder.Err
// 	pa.Events = make([]*Payload, len(txs))
// 	for idx, tx := range txs {
// 		pa.Events[idx], err = DecodeEvent(tx, err)
// 	}
// 	return nil, pa, err
// }

func DecodePayload(raw map[string]interface{}) (*model.Metadata, []model.Transformable, error) {
	// decodingCount.Inc()

	decoder := utility.ManualDecoder{}
	pa := Payload{
		ServiceName:    decoder.String(raw, "service_name"),
		ServiceVersion: decoder.String(raw, "service_version"),
		Sourcemap:      decoder.String(raw, "sourcemap"),
		BundleFilepath: decoder.String(raw, "bundle_filepath"),
	}
	if decoder.Err != nil {
		// decodingError.Inc()
		return nil, nil, decoder.Err
	}
	return &model.Metadata{}, []model.Transformable{&pa}, nil
}
