package logger

import (
	"fmt"
	"io"
	"log"
	"os"
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

	logLocation := fmt.Sprintf("/var/tmp/%s", logName)
	logFile, err := os.Create(logLocation)
	if err != nil {
		panic(err)
	}

	//logFile := ioutil.Discard

	errLog := log.New(logFile, errPrefix, log.Lshortfile)
	infoLog := log.New(logFile, infoPrefix, log.Lshortfile)
	debugLog := log.New(logFile, debugPrefix, log.Lshortfile)

	return &Log{Err: errLog, Info: infoLog, Debug: debugLog}
}

func CreateStdOutLogger(hostName string, logName string) *Log {
	errPrefix := fmt.Sprintf("\x1b[31m %s \x1b[0m", hostName)
	infoPrefix := fmt.Sprintf("\x1b[32m %s \x1b[0m", hostName)
	debugPrefix := fmt.Sprintf("\x1b[33m %s \x1b[0m", hostName)

	errLog := log.New(os.Stdout, errPrefix, log.Lshortfile)
	infoLog := log.New(os.Stdout, infoPrefix, log.Lshortfile)
	debugLog := log.New(os.Stdout, debugPrefix, log.Lshortfile)

	return &Log{Err: errLog, Info: infoLog, Debug: debugLog}
}

func CreateIOLogger(out io.Writer, hostName string, logName string) *Log {
	errPrefix := fmt.Sprintf("\x1b[31m %s \x1b[0m", hostName)
	infoPrefix := fmt.Sprintf("\x1b[32m %s \x1b[0m", hostName)
	debugPrefix := fmt.Sprintf("\x1b[33m %s \x1b[0m", hostName)

	errLog := log.New(out, errPrefix, log.Lshortfile)
	infoLog := log.New(out, infoPrefix, log.Lshortfile)
	debugLog := log.New(out, debugPrefix, log.Lshortfile)

	return &Log{Err: errLog, Info: infoLog, Debug: debugLog}
}
