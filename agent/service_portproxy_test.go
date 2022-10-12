package agent

import (
	"bytes"
	"io"
	"log"
	"os"
	"testing"
	"time"
)

var hub = NewHub()

const echoAddr = "tcp://localhost:9922"

func init() {
	log.Println("start test server")
	go runEchoServer(echoAddr)
}

func TestPortproxy(t *testing.T) {
	addr := "tcp://localhost:9921"
	p := NewPortproxy(hub, addr, echoAddr, "")
	err := p.Init()
	if err != nil {
		t.Error("init error", err)
		return
	}
	go p.Start()

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

	p.Close()
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
