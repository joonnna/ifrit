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

	"github.com/joonnna/ifrit"
	"github.com/joonnna/ifrit/log"
)

var (
	errNoAddr = errors.New("No certificate authority address provided, can't continue")
)

func main() {
	var caAddr string

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&caAddr, "addr", "", "address(ip:port) of certificate authority")
	args.Parse(os.Args[1:])

	if caAddr == "" {
		panic(errNoAddr)
	}

	f, err := os.Create("/var/log/calog")
	if err != nil {
		panic(err)
	}

	log.Init(f, log.DEBUG)

	c, err := ifrit.NewClient(&ifrit.Config{Ca: true, CaAddr: caAddr})
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
