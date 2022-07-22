package service

import (
	"fmt"
	"log"
	"os"
	"strings"
)

func NewLog(logname string) *log.Logger {
	logname = strings.Trim(logname, " ")
	if logname == "" {
		return nil
	}
	return log.New(os.Stdout, fmt.Sprintf("[%v]", logname), log.LstdFlags)
}
