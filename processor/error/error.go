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
	err "github.com/elastic/apm-server/model/error"
	"github.com/elastic/apm-server/processor"
	"github.com/elastic/beats/libbeat/monitoring"
)

var (
	Processor = &processor.PayloadProcessor{
		ProcessorName: "error",
		DecodePayload: err.DecodePayload,
		PayloadSchema: err.PayloadSchema(),
		DecodingCount: monitoring.NewInt(err.Metrics, "decoding.count"),
		DecodingError: monitoring.NewInt(err.Metrics, "decoding.errors"),
		ValidateCount: monitoring.NewInt(err.Metrics, "validation.count"),
		ValidateError: monitoring.NewInt(err.Metrics, "validation.errors"),
	}
)
