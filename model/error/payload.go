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
	"github.com/elastic/apm-server/model"
	"github.com/elastic/apm-server/utility"
)

func DecodePayload(raw map[string]interface{}) (*model.Metadata, []model.Transformable, error) {
	if raw == nil {
		return nil, nil, nil
	}

	var err error
	var metadata *model.Metadata
	metadata, err = model.DecodeMetadata(raw, err)
	if err != nil {
		return nil, nil, err
	}

	decoder := utility.ManualDecoder{}
	errs := decoder.InterfaceArr(raw, "errors")
	events := make([]model.Transformable, len(errs))
	err = decoder.Err
	for idx, errData := range errs {
		events[idx], err = DecodeEvent(errData, err)
	}
	return metadata, events, err
}
