package log

import (
	"fmt"
	"io"
	"log"
	"sync"
)

var (
	err      *log.Logger
	info     *log.Logger
	debug    *log.Logger
	logLvl   int
	lvlMutex sync.RWMutex
)

const (
	ERROR = 0
	INFO  = 1
	DEBUG = 2
)

// Enables the global logger for ifrit.
// If not called, ifrit will not log anything.
// Everything is logged to the provided io.Writer
// Set levels by using log.ERROR, log.INFO, log.DEBUG
func Init(out io.Writer, level int) {
	errPrefix := fmt.Sprintf("\x1b[31m ERROR \x1b[0m")
	infoPrefix := fmt.Sprintf("\x1b[32m INFO \x1b[0m")
	debugPrefix := fmt.Sprintf("\x1b[33m DEBUG \x1b[0m")

	setLvl(level)

	err = log.New(out, errPrefix, log.Lshortfile)
	info = log.New(out, infoPrefix, log.Lshortfile)
	debug = log.New(out, debugPrefix, log.Lshortfile)
}

func Debug(s string, v ...interface{}) {
	if debug != nil {
		if lvl := getLvl(); lvl == DEBUG {
			debug.Output(2, fmt.Sprintf(s, v...))
		}
	}
}

func Info(s string, v ...interface{}) {
	if info != nil {
		if lvl := getLvl(); lvl == DEBUG || lvl == INFO {
			info.Output(2, fmt.Sprintf(s, v...))
		}
	}
}

func Error(s string, v ...interface{}) {
	if err != nil {
		if lvl := getLvl(); lvl == ERROR || lvl == INFO {
			err.Output(2, fmt.Sprintf(s, v...))
		}
	}
}

func SetLogLevel(level int) {
	setLvl(level)
}

func getLvl() int {
	lvlMutex.RLock()
	defer lvlMutex.RUnlock()

	return logLvl
}

func setLvl(lvl int) {
	lvlMutex.Lock()
	defer lvlMutex.Unlock()

	logLvl = lvl
}
