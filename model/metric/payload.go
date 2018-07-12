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

package metric

import (
	"fmt"
	"time"

	"github.com/pkg/errors"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/model/metric/generated/schema"
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/apm-server/validation"
	"github.com/elastic/beats/libbeat/beat"
	"github.com/elastic/beats/libbeat/common"
	"github.com/elastic/beats/libbeat/logp"
	"github.com/elastic/beats/libbeat/monitoring"
	"github.com/santhosh-tekuri/jsonschema"
)

var (
	transformations = monitoring.NewInt(metricMetrics, "transformations")
	processorEntry  = common.MapStr{"name": "metric", "event": "metric"}
)

var cachedPayloadSchema = validation.CreateSchema(schema.PayloadSchema, "metric")
var cachedModelSchema = validation.CreateSchema(schema.ModelSchema, "metric")

func PayloadSchema() *jsonschema.Schema {
	return cachedPayloadSchema
}

func ModelSchema() *jsonschema.Schema {
	return cachedModelSchema
}

type sample interface {
	transform(common.MapStr) error
}

type metric struct {
	samples   []sample
	tags      common.MapStr
	timestamp time.Time
}

func (me *metric) Events(tctx *m.TransformContext) []beat.Event {
	transformations.Inc()
	if me == nil {
		return nil
	}

	samples := common.MapStr{}
	for _, sample := range me.samples {
		if err := sample.transform(samples); err != nil {
			logp.NewLogger("transform").Warnf("failed to transform sample %#v", sample)
			continue
		}
	}

	context := m.NewContext(tctx).Merge(common.MapStr{})

	if me.tags != nil {
		context["tags"] = me.tags
	}
	ev := beat.Event{
		Fields: common.MapStr{
			"processor": processorEntry,
			"context":   context,
			"metric":    samples,
		},
		Timestamp: me.timestamp,
	}

	return []beat.Event{ev}
}

type metricDecoder struct {
	*utility.ManualDecoder
}

func DecodePayload(raw map[string]interface{}) (*m.Metadata, []m.Transformable, error) {
	if raw == nil {
		return nil, nil, nil
	}

	var err error
	var metadata *m.Metadata
	metadata, err = m.DecodeMetadata(raw, err)
	if err != nil {
		return nil, nil, err
	}

	decoder := utility.ManualDecoder{}
	metrics := decoder.InterfaceArr(raw, "metrics")
	if decoder.Err != nil {
		return nil, nil, decoder.Err
	}

	mes := make([]m.Transformable, len(metrics))
	for idx, metricData := range metrics {
		mes[idx], err = DecodeMetric(metricData, err)
		if err != nil {
			return nil, nil, decoder.Err
		}
	}
	return metadata, mes, decoder.Err
}

func DecodeMetric(input interface{}, err error) (m.Transformable, error) {
	if input == nil {
		return nil, errors.New("no data for metric event")
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("invalid type for metric event")
	}

	decoder := metricDecoder{&utility.ManualDecoder{}}

	metric := metric{
		samples:   decoder.decodeSamples(raw["samples"]),
		timestamp: decoder.TimeRFC3339WithDefault(raw, "timestamp"),
	}
	if tags := utility.Prune(decoder.MapStr(raw, "tags")); len(tags) > 0 {
		metric.tags = tags
	}
	return &metric, nil
}

func (md *metricDecoder) decodeSamples(input interface{}) []sample {
	if input == nil {
		md.Err = errors.New("no samples for metric event")
		return nil
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		md.Err = errors.New("invalid type for samples in metric event")
		return nil
	}

	samples := make([]sample, len(raw))
	i := 0
	for name, s := range raw {
		if s == nil {
			continue
		}
		sampleMap, ok := s.(map[string]interface{})
		if !ok {
			md.Err = fmt.Errorf("invalid sample: %s: %s", name, s)
			return nil
		}

		sampleType, ok := sampleMap["type"]
		if !ok {
			md.Err = fmt.Errorf("missing sample type: %s: %s", name, s)
			return nil
		}

		var sample sample
		switch sampleType {
		case "counter":
			sample = md.decodeCounter(name, sampleMap)
		case "gauge":
			sample = md.decodeGauge(name, sampleMap)
		case "summary":
			sample = md.decodeSummary(name, sampleMap)
		}
		if md.Err != nil {
			return nil
		}
		samples[i] = sample
		i++
	}
	return samples
}
