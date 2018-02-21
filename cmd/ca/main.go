package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/joonnna/ifrit/cauth"
	"github.com/joonnna/ifrit/log"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var numRings uint

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 10, "Number of gossip rings to be used")

	args.Parse(os.Args[1:])

	f, err := os.Create("/var/log/calog")
	if err != nil {
		panic(err)
	}

	log.Init(f, log.DEBUG)

	ca, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}

	go ca.Start(uint32(numRings))

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	ca.Shutdown()
}
