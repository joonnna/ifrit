package logger

import (
	"log"
	"fmt"
	"os"
)

type Log struct {
	Err *log.Logger
	Info *log.Logger
	Debug *log.Logger
}

func CreateLogger(hostName string, logName string) *Log {
	errPrefix := fmt.Sprintf("\x1b[31m %s \x1b[0m", hostName)
	infoPrefix := fmt.Sprintf("\x1b[32m %s \x1b[0m", hostName)
	debugPrefix := fmt.Sprintf("\x1b[33m %s \x1b[0m", hostName)

	logLocation := fmt.Sprintf("/var/tmp/%s", logName)

	logFile, _ := os.OpenFile(logLocation, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)

	errLog := log.New(logFile, errPrefix, log.Lshortfile)
	infoLog := log.New(logFile, infoPrefix, log.Lshortfile)
	debugLog := log.New(logFile, debugPrefix, log.Lshortfile)

	return &Log{Err: errLog, Info: infoLog, Debug: debugLog}
}
