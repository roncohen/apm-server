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

package model

import (
	"github.com/elastic/apm-server/model/generated/schema"
	"github.com/elastic/apm-server/validation"
	"github.com/santhosh-tekuri/jsonschema"
)

type Metadata struct {
	Service *Service
	Process *Process
	System  *System
	User    *User
}

var cachedModelSchema = validation.CreateSchema(schema.MetadataSchema, "metadata")

func MetadataSchema() *jsonschema.Schema {
	return cachedModelSchema
}

func DecodeMetadata(raw map[string]interface{}, err error) (*Metadata, error) {
	if raw == nil || len(raw) == 0 {
		return nil, nil
	}

	var metadata Metadata
	metadata.Service, err = DecodeService(raw["service"], err)
	metadata.Process, err = DecodeProcess(raw["process"], err)
	metadata.System, err = DecodeSystem(raw["system"], err)
	metadata.User, err = DecodeUser(raw["user"], err)

	return &metadata, err
}
