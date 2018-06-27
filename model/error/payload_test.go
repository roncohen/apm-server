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

package error

import (
	"errors"
	"fmt"
	"testing"

	"github.com/elastic/apm-server/model"

	"github.com/stretchr/testify/assert"

	"time"

	m "github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/sourcemap"
	"github.com/elastic/beats/libbeat/common"
)

func TestPayloadDecode(t *testing.T) {
	timestamp := "2017-05-30T18:53:27.154Z"
	timestampParsed, _ := time.Parse(time.RFC3339, timestamp)
	pid, ip := 1, "127.0.0.1"
	for idx, test := range []struct {
		input    map[string]interface{}
		err      error
		event    *Event
		metadata model.Metadata
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
			input: map[string]interface{}{"user": 123},
			err:   errors.New("Invalid type for user"),
		},
		{
			input:    map[string]interface{}{},
			err:      nil,
			event:    nil,
			metadata: model.Metadata{},
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
				"user":    map[string]interface{}{"ip": ip},
				"errors": []interface{}{
					map[string]interface{}{
						"timestamp": timestamp,
						"exception": map[string]interface{}{
							"message": "Exception Msg",
						},
					},
				},
			},
			err: nil,
			event: &Event{Timestamp: timestampParsed,
				Exception: &Exception{Message: "Exception Msg", Stacktrace: m.Stacktrace{}},
			},
			metadata: model.Metadata{
				Service: &m.Service{
					Name: "a", Agent: m.Agent{Name: "ag", Version: "1.0"}},
				System:  &m.System{IP: &ip},
				Process: &m.Process{Pid: pid},
				User:    &m.User{IP: &ip},
			},
		},
	} {
		metadata, events, err := DecodePayload(test.input)
		if test.err != nil {
			assert.Equal(t, test.err, err)
			continue
		}

		if err != nil {
			assert.Fail(t, "Failed at index %v: ", idx, test.err.Error())
			continue
		}

		if test.event != nil {
			assert.Len(t, events, 1)
			assert.Equal(t, test.event, events[0], "Failed at index %v: ", idx)
			assert.Equal(t, test.metadata, *metadata, "Failed at index %v: ", idx)
		}
	}
}

func TestPayloadTransform(t *testing.T) {
	svc := &m.Service{Name: "myservice"}
	timestamp := time.Now()
	// context := common.MapStr{
	// 	"context": common.MapStr{
	// 		"service": common.MapStr{
	// 			"agent": common.MapStr{"name": "", "version": ""},
	// 			"name":  "myservice",
	// 		},
	// 	},
	// }

	tests := []struct {
		Metadata model.Metadata
		Event    *Event
		Output   common.MapStr
		Msg      string
	}{
		// {
		// 	Metadata:       model.Metadata{Service: svc},
		// 	Transformables: []model.Transformable{},
		// 	Output:         nil,
		// 	Msg:            "Empty Event Array",
		// },
		{
			Metadata: model.Metadata{
				Service: svc,
			},
			Event: &Event{
				Timestamp: timestamp,
			},
			Output: common.MapStr{
				"error": common.MapStr{
					"grouping_key": "d41d8cd98f00b204e9800998ecf8427e",
				},
				"processor": common.MapStr{"event": "error", "name": "error"},
				"context": common.MapStr{
					"service": common.MapStr{
						"name":  "myservice",
						"agent": common.MapStr{"name": "", "version": ""},
					},
				},
			},
			Msg: "Payload with valid Event.",
		},
		{
			Metadata: model.Metadata{Service: svc},
			Event: &Event{
				Timestamp: timestamp,
				Context:   common.MapStr{"foo": "bar", "user": common.MapStr{"email": "m@m.com"}},
				Log:       baseLog(),
				Exception: &Exception{
					Message:    "exception message",
					Stacktrace: m.Stacktrace{&m.StacktraceFrame{Filename: "myFile"}},
				},
				Transaction: &Transaction{Id: "945254c5-67a5-417e-8a4e-aa29efcbfb79"},
			},
			Output: common.MapStr{
				"context": common.MapStr{
					"foo": "bar", "user": common.MapStr{"email": "m@m.com"},
					"service": common.MapStr{
						"name":  "myservice",
						"agent": common.MapStr{"name": "", "version": ""},
					},
				},
				"error": common.MapStr{
					"grouping_key": "1d1e44ffdf01cad5117a72fd42e4fdf4",
					"log":          common.MapStr{"message": "error log message"},
					"exception": common.MapStr{
						"message": "exception message",
						"stacktrace": []common.MapStr{{
							"exclude_from_grouping": false,
							"filename":              "myFile",
							"line":                  common.MapStr{"number": 0},
							"sourcemap": common.MapStr{
								"error":   "Colno mandatory for sourcemapping.",
								"updated": false,
							},
						}},
					},
				},
				"processor":   common.MapStr{"event": "error", "name": "error"},
				"transaction": common.MapStr{"id": "945254c5-67a5-417e-8a4e-aa29efcbfb79"},
			},
			Msg: "Payload with Event with Context.",
		},
	}

	for idx, test := range tests {
		tctx := &model.TransformContext{
			Config:   model.TransformConfig{SmapMapper: &sourcemap.SmapMapper{}},
			Metadata: test.Metadata,
		}
		outputEvents := test.Event.Events(tctx)
		assert.Len(t, outputEvents, 1)
		outputEvent := outputEvents[0]
		assert.Equal(t, test.Output, outputEvent.Fields, fmt.Sprintf("Failed at idx %v; %s", idx, test.Msg))
		assert.Equal(t, timestamp, outputEvent.Timestamp, fmt.Sprintf("Bad timestamp at idx %v; %s", idx, test.Msg))
	}
}
