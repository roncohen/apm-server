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
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/beats/libbeat/common"
)

// assertMetricsMatch is an equality test for a metric as sample order is not important
func assertMetricsMatch(t *testing.T, expected, actual metric) bool {
	samplesMatch := assert.ElementsMatch(t, expected.samples, actual.samples)
	expected.samples = nil
	actual.samples = nil
	nonSamplesMatch := assert.Equal(t, expected, actual)

	return assert.True(t, samplesMatch && nonSamplesMatch,
		fmt.Sprintf("metrics mismatch\nexpected:%#v\n   actual:%#v", expected, actual))
}

func TestPayloadDecode(t *testing.T) {
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	pid, ip := 1, "127.0.0.1"
	unit := "foos"
	for idx, test := range []struct {
		input    map[string]interface{}
		err      error
		event    *metric
		metadata *m.Metadata
	}{
		{input: nil, err: nil},
		{
			input: map[string]interface{}{"service": 123},
			err:   errors.New("Invalid type for service"),
		},
		{
			input: map[string]interface{}{"system": 123},
			err:   errors.New("Invalid type for system"),
		},
		{
			input: map[string]interface{}{"process": 123},
			err:   errors.New("Invalid type for process"),
		},
		{
			input: map[string]interface{}{},
			err:   nil,
			// event: &metric{},
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples":   map[string]interface{}{},
					},
				},
			},
			err: nil,
			event: &metric{
				samples:   []sample{},
				tags:      nil,
				timestamp: timestampParsed,
			},
			metadata: &m.Metadata{
				Service: &m.Service{
					Name: "a", Agent: m.Agent{Name: "ag", Version: "1.0"}},
				System:  &m.System{IP: &ip},
				Process: &m.Process{Pid: pid},
			},
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"invalid.counter": map[string]interface{}{
								"type":  "counter",
								"value": "foo",
							},
						},
					},
				},
			},
			err: errors.New("Error fetching field"),
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"invalid.gauge": map[string]interface{}{
								"type":  "gauge",
								"value": "foo",
							},
						},
					},
				},
			},
			err: errors.New("Error fetching field"),
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"empty.metric": map[string]interface{}{},
						},
					},
				},
			},
			err: errors.New("missing sample type"),
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"invalid.key.metric": map[string]interface{}{
								"foo": "bar",
							},
						},
					},
				},
			},
			err: errors.New("missing sample type"),
		},
		{
			input: map[string]interface{}{
				"system": map[string]interface{}{"ip": ip},
				"service": map[string]interface{}{
					"name": "a",
					"agent": map[string]interface{}{
						"name": "ag", "version": "1.0",
					}},
				"process": map[string]interface{}{"pid": 1.0},
				"metrics": []interface{}{
					map[string]interface{}{
						"tags": map[string]interface{}{
							"a.tag": "a.tag.value",
						},
						"timestamp": timestamp,
						"samples": map[string]interface{}{
							"a.counter": map[string]interface{}{
								"type":  "counter",
								"value": json.Number("612"),
								"unit":  unit,
							},
							"some.gauge": map[string]interface{}{
								"type":  "gauge",
								"value": json.Number("9.16"),
							},
						},
					},
				},
			},
			err: nil,
			event: &metric{
				samples: []sample{
					&gauge{
						name:  "some.gauge",
						value: 9.16,
					},
					&counter{
						name:  "a.counter",
						count: 612,
						unit:  &unit,
					},
				},
				tags: common.MapStr{
					"a.tag": "a.tag.value",
				},
				timestamp: timestampParsed,
			},
			metadata: &m.Metadata{
				Service: &m.Service{Name: "a", Agent: m.Agent{Name: "ag", Version: "1.0"}},
				System:  &m.System{IP: &ip},
				Process: &m.Process{Pid: pid},
			},
		},
	} {
		metadata, transformables, err := DecodePayload(test.input)

		if err != nil && test.err == nil {
			assert.Fail(t, err.Error())
			continue
		}

		if test.err != nil {
			assert.Error(t, err)
			continue
		}

		if test.event != nil {
			if len(transformables) != 1 {
				assert.Fail(t, "Failed at idx %v; %v items, should have 1", idx, len(transformables))
				continue
			}

			event := transformables[0].(*metric)
			assertMetricsMatch(t, *test.event, *event)
		}

		assert.Equal(t, test.metadata, metadata)
	}
}

func TestPayloadTransform(t *testing.T) {
	svc := m.Service{Name: "myservice"}
	timestamp := time.Now()
	unit := "bytes"

	tests := []struct {
		Event    metric
		Metadata m.Metadata
		Output   []common.MapStr
		Msg      string
	}{
		{
			Event:    metric{timestamp: timestamp},
			Metadata: m.Metadata{Service: &svc},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"service": common.MapStr{
							"agent": common.MapStr{"name": "", "version": ""},
							"name":  "myservice",
						},
					},
					"metric":    common.MapStr{},
					"processor": common.MapStr{"event": "metric", "name": "metric"},
				},
			},
			Msg: "Payload with empty metric.",
		},
		{
			Event: metric{
				tags:      common.MapStr{"a.tag": "a.tag.value"},
				timestamp: timestamp,
				samples: []sample{
					&counter{
						name:  "a.counter",
						count: 612,
					},
					&gauge{
						name:  "some.gauge",
						value: 9.16,
						unit:  &unit,
					},
				},
			},
			Metadata: m.Metadata{Service: &svc},
			Output: []common.MapStr{
				{
					"context": common.MapStr{
						"service": common.MapStr{
							"name":  "myservice",
							"agent": common.MapStr{"name": "", "version": ""},
						},
						"tags": common.MapStr{
							"a.tag": "a.tag.value",
						},
					},
					"metric": common.MapStr{
						"a.counter":  common.MapStr{"value": float64(612), "type": "counter"},
						"some.gauge": common.MapStr{"value": float64(9.16), "type": "gauge", "unit": unit},
					},
					"processor": common.MapStr{"event": "metric", "name": "metric"},
				},
			},
			Msg: "Payload with valid metric.",
		},
	}

	for idx, test := range tests {
		tctx := m.TransformContext{Metadata: test.Metadata}
		outputEvents := test.Event.Events(&tctx)
		if len(outputEvents) != len(test.Output) {
			t.Errorf("Failed at idx %v; output has %v events, expected %v", idx, len(outputEvents), len(test.Output))
			continue
		}

		for j, outputEvent := range outputEvents {
			assert.Equal(t, test.Output[j], outputEvent.Fields, "Failed at idx %v; %s", idx, test.Msg)
			assert.Equal(t, timestamp, outputEvent.Timestamp, "Bad timestamp at idx %v; %s", idx, test.Msg)
		}
	}
}
