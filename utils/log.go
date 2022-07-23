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
}

func NewNamedLogger(name string, asyncOutput bool) *NamedLogger {
	asyncOutput = false
	name = strings.Trim(name, " ")
	if name == "" {
		return &NamedLogger{
			asyncOutput: asyncOutput,
		}
	}

	return &NamedLogger{
		logger:      log.New(logOutputDist, fmt.Sprintf("[%v]", name), log.LstdFlags),
		name:        name,
		asyncOutput: asyncOutput,
	}
}

func SetNamedLoggerOutputDist(dist *os.File) {
	logOutputDist = dist
}

func (nl *NamedLogger) Printf(format string, v ...interface{}) {
	if nl.logger == nil {
		return
	}

	if nl.asyncOutput {
		go nl.logger.Output(2, fmt.Sprintf(format, v...))
	} else {
		nl.logger.Output(2, fmt.Sprintf(format, v...))
	}
}

func (nl *NamedLogger) Println(v ...interface{}) {
	if nl.logger == nil {
		return
	}

	if nl.asyncOutput {
		go nl.logger.Output(2, fmt.Sprintln(v...))
	} else {
		nl.logger.Output(2, fmt.Sprintln(v...))
	}
}

func (nl *NamedLogger) Fatal(v ...interface{}) {
	if nl.logger != nil {
		if nl.asyncOutput {
			go nl.logger.Output(2, fmt.Sprint(v...))
		} else {
			nl.logger.Output(2, fmt.Sprint(v...))
		}
	}

	os.Exit(1)
}
