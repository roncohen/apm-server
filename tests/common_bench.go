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

package tests

import (
	"testing"

	"github.com/elastic/apm-server/beater"
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/validation"

	"github.com/elastic/apm-server/tests/loader"
)

func benchmarkValidate(b *testing.B, routeModel beater.V1PayloadType, requestInfo RequestInfo) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		data, err := loader.LoadData(requestInfo.Path)
		if err != nil {
			b.Error(err)
		}
		b.StartTimer()
		if err := validation.Validate(data, routeModel.Schema); err != nil {
			b.Error(err)
		}
	}
}

func benchmarkDecode(b *testing.B, routeModel beater.V1PayloadType, requestInfo RequestInfo) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		data, err := loader.LoadData(requestInfo.Path)
		if err != nil {
			b.Error(err)
		}
		b.StartTimer()
		if _, _, err := routeModel.PayloadDecoder(data); err != nil {
			b.Error(err)
		}
	}
}

func benchmarkTransform(b *testing.B, routeModel beater.V1PayloadType, tctx *model.TransformContext, requestInfo RequestInfo) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		data, err := loader.LoadData(requestInfo.Path)
		if err != nil {
			b.Error(err)
		}
		if metadata, transformable, err := routeModel.PayloadDecoder(data); err != nil {
			b.Error(err)
		} else {
			tctx.Metadata = *metadata

			b.StartTimer()
			for _, entity := range transformable {
				entity.Events(tctx)
			}
		}
	}
}

func benchmarkProcessRequest(b *testing.B, routeModel beater.V1PayloadType, tctx *model.TransformContext, requestInfo RequestInfo) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		data, err := loader.LoadData(requestInfo.Path)
		if err != nil {
			b.Error(err)
		}
		b.StartTimer()
		if err := validation.Validate(data, routeModel.Schema); err != nil {
			b.Error(err)
		}
		if metadata, transformables, err := routeModel.PayloadDecoder(data); err != nil {
			b.Error(err)
		} else {
			tctx.Metadata = *metadata
			for _, transformable := range transformables {
				transformable.Events(tctx)
			}
		}
	}
}

func BenchmarkProcessRequests(b *testing.B, routeModel beater.V1PayloadType, tctx *model.TransformContext, requestInfo []RequestInfo) {
	for _, info := range requestInfo {
		validate := func(b *testing.B) {
			benchmarkValidate(b, routeModel, info)
		}
		decode := func(b *testing.B) {
			benchmarkDecode(b, routeModel, info)
		}
		transform := func(b *testing.B) {
			benchmarkTransform(b, routeModel, tctx, info)
		}
		processRequest := func(b *testing.B) {
			benchmarkProcessRequest(b, routeModel, tctx, info)
		}
		b.Run(info.Name+"Validate", validate)
		b.Run(info.Name+"Decode", decode)
		b.Run(info.Name+"Transform", transform)
		b.Run(info.Name+"ProcessRequest", processRequest)
	}
}
