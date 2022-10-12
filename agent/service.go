package agent

type Service interface {
	Network() string
	Init() error
	Start() error
	Close() error
	Update() error // 依赖的netnode重连后，能够更新runner
	ServiceInfo
}

type ServiceInfo interface {
	SetName(name string)
	GetName() string
	SetState(state string)
	GetState() string
	SetID(id int32)
	GetID() int32
	AddActiveCount(n int32)
	AddDoneCount(n int32)
	SetListenAndTarget(l, t string)
	Detail() ServiceDetail
}

type ServiceDetails []ServiceDetail

type ServiceDetail struct {
	Name    string
	Type    string
	State   string
	Listen  string
	Target  string
	Actives int32
	Dones   int32
}
