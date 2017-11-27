package logger

import (
	"fmt"
	"io/ioutil"
	"log"
)

type Log struct {
	Err   *log.Logger
	Info  *log.Logger
	Debug *log.Logger
}

func CreateLogger(hostName string, logName string) *Log {
	errPrefix := fmt.Sprintf("\x1b[31m %s \x1b[0m", hostName)
	infoPrefix := fmt.Sprintf("\x1b[32m %s \x1b[0m", hostName)
	debugPrefix := fmt.Sprintf("\x1b[33m %s \x1b[0m", hostName)

	/*
		logLocation := fmt.Sprintf("/var/tmp/%s", logName)
			logFile, err := os.Create(logLocation)
			if err != nil {
				panic(err)
			}
	*/
	logFile := ioutil.Discard

	errLog := log.New(logFile, errPrefix, log.Lshortfile)
	infoLog := log.New(logFile, infoPrefix, log.Lshortfile)
	debugLog := log.New(logFile, debugPrefix, log.Lshortfile)

	return &Log{Err: errLog, Info: infoLog, Debug: debugLog}
}
