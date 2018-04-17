package model

import (
	"errors"
	"log"

	"github.com/elastic/apm-server/utility"
	"github.com/elastic/beats/libbeat/common"
)

type TransformContext struct {
	Service *Service
	Process *Process
	System  *System
	User    *User

	// cached transformed values
	service *common.MapStr
	process *common.MapStr
	system  *common.MapStr
	user    *common.MapStr
}

// type Header struct {
// 	Service *Service
// 	Process *Process
// 	System  *System
// }

// func NewTransformContext(service *Service, process *Process, system *System, user *User) *TransformContext {
// 	return &TransformContext{
// 		service: service.Transform(),
// 		process: process.Transform(),
// 		system:  system.Transform(),
// 		user:    user.Transform(),
// 	}
// }

func (c *TransformContext) TransformInto(m common.MapStr) common.MapStr {
	if c.service == nil {
		service := c.Service.Transform()
		process := c.Process.Transform()
		system := c.System.Transform()
		user := c.User.Transform()

		c.service = &service
		c.process = &process
		c.system = &system
		c.user = &user
	}

	if m == nil {
		m = common.MapStr{}
	} else {
		for k, v := range m {
			// normalize map entries by calling utility.Add
			utility.Add(m, k, v)
		}
	}

	log.Println("context.system: ", c.System)
	a, _ := c.system.GetValue("hostname")
	log.Println("context.system.pid: ", a)

	a, _ = m.GetValue("context.system")
	log.Println("context.system.pid1: ", a)

	utility.Add(m, "service", *c.service)
	utility.Add(m, "process", *c.process)
	utility.Add(m, "system", *c.system)
	utility.MergeAdd(m, "user", *c.user)

	log.Println("m: ", m)
	return m
}

func DecodeContext(input interface{}, err error) (*TransformContext, error) {
	if input == nil || err != nil {
		return nil, err
	}
	raw, ok := input.(map[string]interface{})
	if !ok {
		return nil, errors.New("Invalid type for header")
	}

	tc := TransformContext{}
	tc.Process, err = DecodeProcess(raw["process"], err)
	tc.Service, err = DecodeService(raw["service"], err)
	tc.System, err = DecodeSystem(raw["system"], err)

	return &tc, err
}
