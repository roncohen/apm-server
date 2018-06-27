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
	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type Context struct {
	service common.MapStr
	process common.MapStr
	system  common.MapStr
	user    common.MapStr
}

func NewContext(tctx *TransformContext) *Context {
	return &Context{
		service: tctx.Metadata.Service.Transform(tctx),
		process: tctx.Metadata.Process.Transform(tctx),
		system:  tctx.Metadata.System.Transform(tctx),
		user:    tctx.Metadata.User.Transform(tctx),
	}
}

func (c *Context) Merge(m common.MapStr) common.MapStr {
	if m == nil {
		m = common.MapStr{}
	} else {
		for k, v := range m {
			// normalize map entries by calling utility.Add
			utility.Add(m, k, v)
		}
	}
	utility.Add(m, "service", c.service)
	utility.Add(m, "process", c.process)
	utility.Add(m, "system", c.system)
	utility.MergeAdd(m, "user", c.user)
	return m
}

// func (c *Context) Transform(tcxt *TransformContext) common.MapStr {
// 	m := common.MapStr{
// 		"service": c.service.Transform(tcxt),
// 		"process: c.process.Transform(tcxt),
// 		"system":  c.system.Transform(tcxt),
// 		"user":    c.user.Transform(tcxt),
// 	}

// 	if m == nil {
// 		m = common.MapStr{}
// 	} else {
// 	for k, v := range m {
// 		// normalize map entries by calling utility.Add
// 		utility.Add(m, k, v)
// 	}
// utility.Add(m, "service", c.service)
// utility.Add(m, "process", c.process)
// utility.Add(m, "system", c.system)
// utility.MergeAdd(m, "user", c.user)
// 	return m
// }
