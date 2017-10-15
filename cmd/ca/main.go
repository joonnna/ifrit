package main

import (
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/joonnna/firechain/cauth"
)

func main() {
	var numRings uint

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 3, "Number of gossip rings to be used")

	args.Parse(os.Args[1:])

	ca, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}

	go ca.Start(uint8(numRings))

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	ca.Shutdown()
}
