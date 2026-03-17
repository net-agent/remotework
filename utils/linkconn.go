package utils

import (
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
)

var (
	linkConnLoggerOnce sync.Once
	linkConnLogger     *slog.Logger
	linkConnAutoID     uint64
)

// writeHalfCloser 表示可以半关闭写端的连接（向对端发送 EOF）。
type writeHalfCloser interface {
	CloseWrite() error
}

// readHalfCloser 表示可以半关闭读端的连接（停止接收数据）。
type readHalfCloser interface {
	CloseRead() error
}

func getLinkConnLogger() *slog.Logger {
	linkConnLoggerOnce.Do(func() {
		linkConnLogger = NewModuleLogger("utils.linkconn")
	})
	return linkConnLogger
}

// RelayConns 双向链接两个可读写关闭的连接。
// 优先使用 half-close 传播单向 EOF，双向传输结束后再全关闭。
//
// 返回值语义：bytesToA 是写入 a 的字节数（b→a 方向），bytesToB 同理。
func RelayConns(a, b io.ReadWriteCloser) (bytesToA, bytesToB int64, err error) {
	logger := getLinkConnLogger()
	linkID := atomic.AddUint64(&linkConnAutoID, 1)

	var wg sync.WaitGroup
	var closeOnce sync.Once
	var errOnce sync.Once
	var firstErr error

	setFirstErr := func(e error) {
		if e == nil || e == io.EOF {
			return
		}
		errOnce.Do(func() {
			firstErr = e
		})
	}

	closeConns := func(reason string) {
		logger.Debug("link closing",
			"id", linkID,
			"reason", reason,
			"a_type", fmt.Sprintf("%T", a),
			"b_type", fmt.Sprintf("%T", b),
		)
		_ = a.Close()
		_ = b.Close()
	}

	relayOneDir := func(src, dst io.ReadWriteCloser, direction string, written *int64) {
		defer wg.Done()

		n, copyErr := io.Copy(dst, src)
		*written = n

		logger.Debug("relay finished",
			"id", linkID,
			"dir", direction,
			"bytes", n,
			"err", copyErr,
		)

		if copyErr != nil {
			setFirstErr(copyErr)
			closeOnce.Do(func() { closeConns("copy_error:" + direction) })
			return
		}

		// CloseWrite 失败 → 触发全关闭：无法通知对端 EOF 是致命问题，
		// 对端会永远阻塞在读取上。
		if cw, ok := dst.(writeHalfCloser); ok {
			if e := cw.CloseWrite(); e != nil {
				logger.Debug("close write failed", "id", linkID, "dir", direction, "err", e)
				setFirstErr(e)
				closeOnce.Do(func() { closeConns("close_write_error:" + direction) })
				return
			}
			logger.Debug("close write ok", "id", linkID, "dir", direction)
		} else {
			logger.Debug("close write unsupported, fallback close", "id", linkID, "dir", direction)
			closeOnce.Do(func() { closeConns("no_close_write:" + direction) })
			return
		}

		// CloseRead 失败 → 仅记录，不触发全关闭：此方向数据已读完，
		// 读端关闭失败不影响另一方向的正常传输。
		if cr, ok := src.(readHalfCloser); ok {
			if e := cr.CloseRead(); e != nil {
				logger.Debug("close read failed", "id", linkID, "dir", direction, "err", e)
				setFirstErr(e)
			} else {
				logger.Debug("close read ok", "id", linkID, "dir", direction)
			}
		}
	}

	logger.Debug("link started",
		"id", linkID,
		"a_type", fmt.Sprintf("%T", a),
		"b_type", fmt.Sprintf("%T", b),
	)

	wg.Add(2)
	go relayOneDir(a, b, "a->b", &bytesToB)
	go relayOneDir(b, a, "b->a", &bytesToA)
	wg.Wait()

	closeOnce.Do(func() { closeConns("both_done") })

	logger.Debug("link finished",
		"id", linkID,
		"bytes_to_a", bytesToA,
		"bytes_to_b", bytesToB,
		"err", firstErr,
	)

	return bytesToA, bytesToB, firstErr
}
