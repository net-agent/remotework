package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

var logOutputDist = os.Stdout

type NamedLogger struct {
	logger      *log.Logger
	name        string
	asyncOutput bool
	msgChan     chan string
	closeOnce   sync.Once
}

func NewNamedLogger(name string, asyncOutput bool) *NamedLogger {
	name = strings.Trim(name, " ")
	if name == "" {
		return &NamedLogger{
			asyncOutput: asyncOutput,
		}
	}

	nl := &NamedLogger{
		logger:      log.New(logOutputDist, fmt.Sprintf("[%v]", name), log.LstdFlags),
		name:        name,
		asyncOutput: asyncOutput,
	}

	if asyncOutput {
		nl.msgChan = make(chan string, 1000)
		go nl.startAsyncWorker()
	}

	return nl
}

func (nl *NamedLogger) startAsyncWorker() {
	for msg := range nl.msgChan {
		nl.logger.Output(2, msg)
	}
}

// Close 关闭异步 logger，等待所有消息写入完成
func (nl *NamedLogger) Close() {
	if nl.asyncOutput && nl.msgChan != nil {
		nl.closeOnce.Do(func() {
			close(nl.msgChan)
		})
	}
}

func SetNamedLoggerOutputDist(dist *os.File) {
	logOutputDist = dist
}

func (nl *NamedLogger) Printf(format string, v ...interface{}) {
	if nl.logger == nil {
		return
	}

	msg := fmt.Sprintf(format, v...)
	if nl.asyncOutput {
		// 直接发送到 channel，满时阻塞等待
		nl.msgChan <- msg
	} else {
		nl.logger.Output(2, msg)
	}
}

func (nl *NamedLogger) Println(v ...interface{}) {
	if nl.logger == nil {
		return
	}

	msg := fmt.Sprintln(v...)
	if nl.asyncOutput {
		nl.msgChan <- msg
	} else {
		nl.logger.Output(2, msg)
	}
}

func (nl *NamedLogger) Fatal(v ...interface{}) {
	if nl.logger != nil {
		msg := fmt.Sprint(v...)
		// Fatal should always be synchronous to ensure output before exit
		nl.logger.Output(2, msg)
	}

	os.Exit(1)
}
