package agent

import (
	"errors"
	"net"
	"sync"
	"time"

	"github.com/net-agent/cipherconn"
)

//
//
// Listener
//

type secretListener struct {
	net.Listener
	ch chan net.Conn
}

func newSecretListener(l net.Listener, secret string) net.Listener {
	ch := make(chan net.Conn, 128)
	go func() {
		var wg sync.WaitGroup
		for {
			conn, err := l.Accept()
			if err != nil {
				break
			}

			wg.Add(1)
			go func(c net.Conn) {
				defer wg.Done()
				cc, err := cipherconn.New(c, secret)
				if err != nil {
					c.Close()
					return
				}
				select {
				case ch <- cc:
				case <-time.After(time.Second * 20):
				}
			}(conn)
		}
		wg.Wait() // wait all channel push done
		close(ch)
	}()

	sl := &secretListener{
		Listener: l,
		ch:       ch,
	}

	return sl
}

func (l *secretListener) Accept() (net.Conn, error) {
	c, ok := <-l.ch
	if !ok {
		return nil, errors.New("listener closed")
	}
	return c, nil
}
