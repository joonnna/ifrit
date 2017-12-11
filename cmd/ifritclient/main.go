package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	"net/http"
	_ "net/http/pprof"

	"github.com/joonnna/ifrit/client"
)

var (
	errNoAddr = errors.New("No certificate authority address provided, can't continue")
)

func main() {
	var caAddr string
	var cpuprofile, memprofile string

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&caAddr, "addr", "", "address(ip:port) of certificate authority")
	args.StringVar(&cpuprofile, "cpuprofile", "", "write cpu profile `file`")
	args.StringVar(&memprofile, "memprofile", "", "write memory profile to `file`")

	args.Parse(os.Args[1:])

	if cpuprofile != "" {
		f, err := os.Create(cpuprofile)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
		defer pprof.StopCPUProfile()

		go http.ListenAndServe(":8080", nil)
	}

	if memprofile != "" {
		f, err := os.Create(memprofile)
		if err != nil {
			log.Fatal("could not create memory profile: ", err)
		}

		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			log.Fatal("could not write memory profile: ", err)
		}
		f.Close()
	}

	if caAddr == "" {
		panic(errNoAddr)
	}

	c, err := client.NewClient(caAddr)
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
