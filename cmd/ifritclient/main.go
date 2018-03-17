package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	_ "net/http/pprof"

	"github.com/jinzhu/configor"
	"github.com/joonnna/ifrit"

	log "github.com/inconshreveable/log15"
)

var (
	errNoAddr = errors.New("No certificate authority address provided, can't continue")
	logger    = log.New("module", "ifritclient/main")
)

func main() {
	var caAddr string

	runtime.GOMAXPROCS(runtime.NumCPU())

	configor.New(&configor.Config{ENVPrefix: "IFRIT"}).Load(&ClientConfig, "/etc/ifrit/client/config.yml", "config.yml")

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&caAddr, "addr", ClientConfig.Host, "address(ip:port) of certificate authority")
	args.StringVar(&ClientConfig.LogFile, "logfile", ClientConfig.LogFile, "Log to file.")

	args.Parse(os.Args[1:])
	/*
		if caAddr == "" {
			panic(errNoAddr)
		}
	*/

	if ClientConfig.LogFile != "" {
		r := log.Root()
		h := log.CallerFileHandler(log.Must.FileHandler(ClientConfig.LogFile, log.LogfmtFormat()))
		r.SetHandler(h)
	}

	c, err := ifrit.NewClient(nil) //&ifrit.Config{Ca: true, CaAddr: caAddr})
	if err != nil {
		fmt.Println(err)
		panic(err)
	}

	go c.Start()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	c.ShutDown()
}
