package agent

import "sync/atomic"

type networkinfo struct {
	name        string
	dialCount   int32
	listenCount int32
}

func (info *networkinfo) GetName() string        { return info.name }
func (info *networkinfo) addDialCount(n int32)   { atomic.AddInt32(&info.dialCount, n) }
func (info *networkinfo) addListenCount(n int32) { atomic.AddInt32(&info.listenCount, n) }
