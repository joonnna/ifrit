package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/cauth"
)

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var numRings uint

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 10, "Number of gossip rings to be used")

	args.Parse(os.Args[1:])

	r := log.Root()

	h := log.CallerFileHandler(log.Must.FileHandler("/var/log/ifritlog", log.TerminalFormat()))

	r.SetHandler(h)

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
