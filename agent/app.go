package agent

type Hub interface {
	Add(name string, it Runner) error
	Del(name string) error

	Get(name string) (Runner, error)
	Return(name string) error

	Report(name string, heads []string) ([][]string, error)
}

type Runner interface {
	Init()
	Start()
	Stop()
	Clean()

	Report(heads []string) ([]string, error)
}
