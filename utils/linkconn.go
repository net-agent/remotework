package utils

import (
	"io"
	"time"
)

func LinkReadWriter(dist, src io.ReadWriter) (distReaded, distWritten int64, retErr error) {
	errChan := make(chan error, 2)

	go func() {
		var err error
		distReaded, err = io.Copy(dist, src)
		errChan <- err
	}()

	go func() {
		var err error
		distWritten, err = io.Copy(src, dist)
		errChan <- err
	}()

	// 等待第一个错误返回
	err := <-errChan

	// 等待第二个错误返回，1秒内不返回则忽略
	select {
	case <-errChan:
	case <-time.After(time.Second * 1):
	}

	if err != nil && err != io.EOF {
		retErr = err
	}

	return
}
