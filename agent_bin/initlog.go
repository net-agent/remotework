package main

import (
	"fmt"
	"log"
	"os"
	"time"
)

func initLogOutput() (f *os.File, shouldClose bool) {
	if FileExist("./temp") {
		fpath := fmt.Sprintf("./temp/agent_%v.log", time.Now().Format("20060102_150405"))
		fmt.Printf("write log to file: %v\n", fpath)
		f, err := os.OpenFile(fpath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err == nil {
			log.SetOutput(f)
			return f, true
		}
	}

	// log.SetOutput(os.Stderr)
	return os.Stderr, false
}

func FileExist(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}
