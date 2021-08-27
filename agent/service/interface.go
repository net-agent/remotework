package service

type Service interface {
	Init() error
	Start() error
	Close() error
}
