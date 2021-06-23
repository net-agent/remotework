package main

import (
	"bytes"
	"io"
	"log"
	"net"
	"os"
	"testing"
)

const echoAddr = "localhost:9922"

// const helloAddr = "localhost:9923"

func init() {
	log.Println("start test server")
	go runEchoServer(echoAddr)
}

func TestPortproxy(t *testing.T) {
	addr := "localhost:9921"
	p := NewPortproxy(echoAddr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		t.Error(err)
		return
	}
	go p.Run(l)

	conn, err := net.Dial("tcp", addr)
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
}

func runEchoServer(addr string) {
	l, err := net.Listen("tcp", addr)
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
