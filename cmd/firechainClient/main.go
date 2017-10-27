package main

import (
	"errors"
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/joonnna/firechain/client"
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

	c := client.NewClient(caAddr)

	go c.Start()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	c.ShutDown()
}
