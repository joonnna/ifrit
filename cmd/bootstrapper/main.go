package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"

	_ "net/http/pprof"

	"github.com/joonnna/ifrit/ifrit"
)

func main() {
	var numRings uint
	var cpuprofile, memprofile string

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 3, "Number of gossip rings to be used")
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

	l, err := ifrit.NewLauncher(uint32(numRings))
	if err != nil {
		panic(err)
	}

	go l.Start()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	l.ShutDown()
}
