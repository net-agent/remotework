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

var mnet = agent.NewNetwork(nil)

const echoAddr = "tcp://localhost:9922"

func init() {
	log.Println("start test server")
	go runEchoServer(echoAddr)
}

func TestPortproxy(t *testing.T) {
	addr := "tcp://localhost:9921"
	p := NewPortproxy(mnet, agent.ServiceInfo{
		Enable: true,
		Param: map[string]string{
			"listen": addr,
			"target": echoAddr,
		},
	})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		p.Run()
		wg.Done()
	}()

	<-time.After(time.Second)
	conn, err := mnet.DialURL(addr)
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
	l, err := mnet.ListenURL(addr)
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
