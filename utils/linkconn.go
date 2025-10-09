package utils

import (
	"io"
	"sync"
)

// LinkReadWriteCloser 双向链接两个可读写关闭的连接
// 当任何一端断开或出错时，会关闭两个连接并返回。
func LinkReadWriteCloser(a, b io.ReadWriteCloser) (aWrittenBytes, bWrittenBytes int64, err error) {
	var wg sync.WaitGroup
	var once sync.Once
	var errOnce sync.Once // 用于记录第一个发生的错误
	var firstErr error

	// 定义一个只执行一次的关闭函数
	closeConns := func() {
		a.Close()
		b.Close()
	}

	// 定义一个只记录一次错误的函数
	setFirstErr := func(e error) {
		if e != nil && e != io.EOF {
			errOnce.Do(func() {
				firstErr = e
			})
		}
	}

	wg.Add(2)

	// Goroutine 1: a -> b
	go func() {
		defer wg.Done()
		// 使用 once.Do 确保一旦这个方向的拷贝结束，两个连接都会被关闭
		defer once.Do(closeConns)

		written, e := io.Copy(b, a)
		bWrittenBytes = written
		setFirstErr(e)
	}()

	// Goroutine 2: b -> a
	go func() {
		defer wg.Done()
		// 同样，使用 once.Do 确保连接被关闭
		defer once.Do(closeConns)

		written, e := io.Copy(a, b)
		aWrittenBytes = written
		setFirstErr(e)
	}()

	// 等待两个 goroutine 都完成
	wg.Wait()

	return aWrittenBytes, bWrittenBytes, firstErr
}
