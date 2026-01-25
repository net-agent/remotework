package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
)

var logOutputDist = os.Stdout

type NamedLogger struct {
	logger      *log.Logger
	name        string
	asyncOutput bool
	msgChan     chan string
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
		nl.msgChan = make(chan string, 1000) // Buffered channel
		go nl.startAsyncWorker()
	}

	return nl
}

func (nl *NamedLogger) startAsyncWorker() {
	for msg := range nl.msgChan {
		nl.logger.Output(2, msg)
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
		select {
		case nl.msgChan <- msg:
		default:
			// Let's try to write to channel, if full, write synchronously to avoid data loss but accept latency penalty?
			// Or just simple send for now.
			nl.msgChan <- msg
		}
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
