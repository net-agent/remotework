package service

import (
	"errors"
	"io"
	"log"
	"net/rpc"

	"github.com/net-agent/flex"
	"github.com/net-agent/remotework/agent"
	"github.com/net-agent/remotework/agent/netx"
	"github.com/net-agent/remotework/rpc/notifyclient"
)

type MessageService struct {
	info   agent.ServiceInfo
	closer io.Closer
}

func NewMessageService(info agent.ServiceInfo) *MessageService {
	return &MessageService{info: info}
}

func (ms *MessageService) Info() string {
	return "message service"
}

func (ms *MessageService) Run() error {
	if !ms.info.Enable {
		return errors.New("service disabled")
	}

	svc := rpc.NewServer()
	svc.Register(notifyclient.New())

	l, err := netx.Listen("flex://0:15")
	if err != nil {
		return err
	}
	ms.closer = l

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Print("rpc.Serve: accept:", err.Error())
			return err
		}

		if stream, ok := conn.(*flex.Stream); ok {
			log.Printf("rpc: %v called\n", stream.Dialer())
		}

		go svc.ServeConn(conn)
	}
}

func (ms *MessageService) Close() error {
	if ms.closer != nil {
		return ms.closer.Close()
	}
	return nil
}
