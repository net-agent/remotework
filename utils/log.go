package utils

import (
	"fmt"
	"log"
	"os"
	"strings"
)

type NamedLogger struct {
	logger *log.Logger
	name   string
}

func NewNamedLogger(name string) *NamedLogger {
	name = strings.Trim(name, " ")
	if name == "" {
		return &NamedLogger{}
	}

	return &NamedLogger{
		logger: log.New(os.Stdout, fmt.Sprintf("[%v]", name), log.LstdFlags),
		name:   name,
	}
}

func (nl *NamedLogger) Printf(format string, v ...interface{}) {
	if nl.logger == nil {
		return
	}

	nl.logger.Output(2, fmt.Sprintf(format, v...))
}

func (nl *NamedLogger) Println(v ...interface{}) {
	if nl.logger == nil {
		return
	}

	nl.logger.Output(2, fmt.Sprintln(v...))
}
