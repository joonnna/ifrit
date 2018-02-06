package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	_ "net/http/pprof"

	"github.com/joonnna/ifrit"
	"github.com/joonnna/ifrit/bootstrap"
)

func createClients(requestChan chan interface{}, exitChan chan bool, arg string, vizAddr string) {
	for {
		select {
		case <-requestChan:
			conf := &ifrit.Config{Visualizer: true, VisAddr: vizAddr}
			c, err := ifrit.NewClient(arg, conf)
			if err != nil {
				fmt.Println(err)
				continue
			}
			requestChan <- c
		case <-exitChan:
			return
		}

	}
}

func main() {
	var numRings uint
	var vizAddr string

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 3, "Number of gossip rings to be used")
	args.StringVar(&vizAddr, "vizAddr", "127.0.0.1:8095", "Address of the visualizer (ip:port)")

	args.Parse(os.Args[1:])

	ch := make(chan interface{})
	exitChan := make(chan bool)

	l, err := bootstrap.NewLauncher(uint32(numRings), ch)
	if err != nil {
		panic(err)
	}

	go createClients(ch, exitChan, l.EntryAddr, vizAddr)
	go l.Start()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	l.ShutDown()
	close(exitChan)
}
