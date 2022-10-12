package agent

import (
	"sync/atomic"
)

type svcinfo struct {
	id      int32
	svctype string
	state   string
	name    string

	_actives int32
	_dones   int32
	listen   string
	target   string
}

func (s *svcinfo) SetID(id int32)         { s.id = id }
func (s *svcinfo) GetID() int32           { return s.id }
func (s *svcinfo) SetState(st string)     { s.state = st }
func (s *svcinfo) GetState() string       { return s.state }
func (s *svcinfo) SetName(name string)    { s.name = name }
func (s *svcinfo) GetName() string        { return s.name }
func (s *svcinfo) AddActiveCount(n int32) { atomic.AddInt32(&s._actives, 1) }
func (s *svcinfo) AddDoneCount(n int32) {
	atomic.AddInt32(&s._actives, -1)
	atomic.AddInt32(&s._dones, 1)
}
func (s *svcinfo) SetListenAndTarget(l, t string) { s.listen = l; s.target = t }
func (s *svcinfo) Detail() ServiceDetail {
	detail := ServiceDetail{
		Name:    s.name,
		Type:    s.svctype,
		State:   s.state,
		Listen:  s.listen,
		Target:  s.target,
		Actives: s._actives,
		Dones:   s._dones,
	}
	return detail
}

func svcName(logName, defaultName string) string {
	if logName != "" {
		return logName
	}
	if defaultName != "" {
		return defaultName
	}
	return "default-svc"
}
