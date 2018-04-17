package stream

import (
	m "github.com/elastic/apm-server/model"
)

type Header struct {
	Service m.Service
	System  *m.System
	Process *m.Process
	User    *m.User
}

func DecodeHeader(raw map[string]interface{}) (*Header, error) {
	if raw == nil {
		return nil, nil
	}
	header := &Header{}

	var err error
	service, err := m.DecodeService(raw["service"], err)
	if service != nil {
		header.Service = *service
	}
	header.System, err = m.DecodeSystem(raw["system"], err)
	header.Process, err = m.DecodeProcess(raw["process"], err)
	header.User, err = m.DecodeUser(raw["user"], err)
	if err != nil {
		return nil, err
	}

	return header, err
}
