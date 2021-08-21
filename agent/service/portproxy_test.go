package service

import (
	"bytes"
	"io"
	"log"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/net-agent/remotework/agent"
)

var hub = agent.NewNetHub()

const echoAddr = "tcp://localhost:9922"

func init() {
	log.Println("start test server")
	go runEchoServer(echoAddr)
}

func TestPortproxy(t *testing.T) {
	addr := "tcp://localhost:9921"
	p := NewPortproxy(hub, agent.ServiceInfo{
		Enable: true,
		Param: map[string]string{
			"listen": addr,
			"target": echoAddr,
		},
	})
	var wg sync.WaitGroup
	err := p.Start(&wg)
	if err != nil {
		t.Error("start failed", err)
		return
	}

	<-time.After(time.Second)
	conn, err := hub.DialURL(addr)
	if err != nil {
		t.Error(err)
		return
	}

	payload := []byte("hello world")

	// write
	go func() {
		_, err := conn.Write(payload)
		if err != nil {
			t.Error(err)
			return
		}
	}()

	buf := make([]byte, len(payload))
	_, err = io.ReadFull(conn, buf)
	if err != nil {
		t.Error(err)
		return
	}
	if !bytes.Equal(buf, payload) {
		t.Error("not equal")
		return
	}

	p.closer.Close()
	wg.Wait()
}

func runEchoServer(addr string) {
	l, err := hub.ListenURL(addr)
	if err != nil {
		os.Exit(-1)
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			os.Exit(-1)
		}

		go io.Copy(conn, conn)
	}
}
