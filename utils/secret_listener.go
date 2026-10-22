package utils

import (
	"net"
	"sync"
	"time"

	"github.com/net-agent/cipherconn"
)

const (
	// secretHandshakeConcurrency 限制并发握手数，避免大量半开连接耗尽 goroutine/fd
	secretHandshakeConcurrency = 128
	// secretHandshakeTimeout 是单条连接完成密钥协商的时间上限
	secretHandshakeTimeout = 10 * time.Second
	// secretDeliverTimeout 是握手完成后等待 Accept 取走连接的时间上限
	secretDeliverTimeout = 20 * time.Second
)

//
//
// Listener
//
//

// secretListener 在 Accept 前完成 cipherconn 密钥协商。
// 底层 Accept 循环独立运行，通过 ch 交付加密连接；Close 会关闭 done、
// 强制关闭握手中的连接并关闭底层 listener，使 Accept 立即返回。
type secretListener struct {
	net.Listener
	ch   chan net.Conn
	done chan struct{}

	// handshakes 记录正在握手的原始连接，Close 时强制关闭以唤醒阻塞的读
	handshakes sync.Map
	closeOnce  sync.Once
	closeErr   error
}

func NewSecretListener(l net.Listener, secret string) net.Listener {
	sl := &secretListener{
		Listener: l,
		ch:       make(chan net.Conn, 128),
		done:     make(chan struct{}),
	}
	go sl.acceptLoop(secret)
	return sl
}

// acceptLoop 接受底层连接并交给 handshake 并发完成握手。
func (l *secretListener) acceptLoop(secret string) {
	sem := make(chan struct{}, secretHandshakeConcurrency)
	var wg sync.WaitGroup

acceptLoop:
	for {
		conn, err := l.Listener.Accept()
		if err != nil {
			break
		}

		select {
		case sem <- struct{}{}:
		case <-l.done:
			_ = conn.Close()
			break acceptLoop
		}

		l.handshakes.Store(conn, struct{}{})
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			defer func() { <-sem }()
			defer l.handshakes.Delete(c)
			l.handshake(c, secret)
		}(conn)
	}

	// 先等所有握手 goroutine 退出（Close 已关闭其连接），再关闭交付通道
	wg.Wait()
	close(l.ch)
	// 丢弃未被 Accept 取走的已加密连接
	for c := range l.ch {
		_ = c.Close()
	}
}

// handshake 完成单条连接的密钥协商，并把结果投递到 ch。
func (l *secretListener) handshake(c net.Conn, secret string) {
	select {
	case <-l.done:
		_ = c.Close()
		return
	default:
	}

	_ = c.SetDeadline(time.Now().Add(secretHandshakeTimeout))
	cc, err := cipherconn.New(c, secret)
	_ = c.SetDeadline(time.Time{}) // clear deadline
	if err != nil {
		_ = c.Close()
		return
	}

	timer := time.NewTimer(secretDeliverTimeout)
	defer timer.Stop()
	select {
	case l.ch <- cc:
	case <-timer.C:
		_ = cc.Close()
	case <-l.done:
		_ = cc.Close()
	}
}

func (l *secretListener) Accept() (net.Conn, error) {
	// Close 优先：避免关闭后仍从缓冲里取走连接
	select {
	case <-l.done:
		return nil, net.ErrClosed
	default:
	}
	select {
	case <-l.done:
		return nil, net.ErrClosed
	case c, ok := <-l.ch:
		if !ok {
			return nil, net.ErrClosed
		}
		return c, nil
	}
}

// Close 幂等：关闭 done 唤醒所有等待者，强关握手中的连接，再关闭底层 listener。
func (l *secretListener) Close() error {
	l.closeOnce.Do(func() {
		close(l.done)
		l.handshakes.Range(func(key, _ any) bool {
			if c, ok := key.(net.Conn); ok {
				_ = c.Close()
			}
			return true
		})
		l.closeErr = l.Listener.Close()
	})
	return l.closeErr
}
