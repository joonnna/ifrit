package logger

import (
	"log"
	"os"
	"strings"
	"fmt"
)

type Log struct {
	Err *log.Logger
	Info *log.Logger
	Debug *log.Logger
}


func CreateLogger() *Log {
	host, _ := os.Hostname()
	hostName := strings.Split(host, ".")[0]

	errPrefix := fmt.Sprintf("\x1b[31m %s \x1b[0m", hostName)
	infoPrefix := fmt.Sprintf("\x1b[32m %s \x1b[0m", hostName)
	debugPrefix := fmt.Sprintf("\x1b[33m %s \x1b[0m", hostName)

	logFile, _ := os.OpenFile("/var/tmp/clientlog", os.O_RDWR|os.O_APPEND|os.O_CREATE, 0660)

	errLog := log.New(logFile, errPrefix, log.Lshortfile)
	infoLog := log.New(logFile, infoPrefix, log.Lshortfile)
	debugLog := log.New(logFile, debugPrefix, log.Lshortfile)

	return &Log{Err: errLog, Info: infoLog, Debug: debugLog}
}
