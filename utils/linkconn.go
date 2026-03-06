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

type closeWriter interface {
	CloseWrite() error
}

type closeReader interface {
	CloseRead() error
}

func getLinkConnLogger() *slog.Logger {
	linkConnLoggerOnce.Do(func() {
		linkConnLogger = NewModuleLogger("utils.linkconn")
	})
	return linkConnLogger
}

// LinkReadWriteCloser 双向链接两个可读写关闭的连接。
// 优先使用 half-close 传播单向 EOF，双向传输结束后再全关闭。
func LinkReadWriteCloser(a, b io.ReadWriteCloser) (aWrittenBytes, bWrittenBytes int64, err error) {
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

	copyPipe := func(src, dst io.ReadWriteCloser, direction string, written *int64) {
		defer wg.Done()

		n, copyErr := io.Copy(dst, src)
		*written = n

		logger.Debug("copy finished",
			"id", linkID,
			"dir", direction,
			"bytes", n,
			"err", copyErr,
		)

		if copyErr != nil && copyErr != io.EOF {
			setFirstErr(copyErr)
			closeOnce.Do(func() { closeConns("copy_error:" + direction) })
			return
		}

		if cw, ok := dst.(closeWriter); ok {
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

		if cr, ok := src.(closeReader); ok {
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
	go copyPipe(a, b, "a->b", &bWrittenBytes)
	go copyPipe(b, a, "b->a", &aWrittenBytes)
	wg.Wait()

	closeOnce.Do(func() { closeConns("both_done") })

	logger.Debug("link finished",
		"id", linkID,
		"a_written", aWrittenBytes,
		"b_written", bWrittenBytes,
		"err", firstErr,
	)

	return aWrittenBytes, bWrittenBytes, firstErr
}
