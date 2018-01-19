package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"sync"
	"syscall"

	_ "net/http/pprof"

	"github.com/joonnna/ifrit/cauth"
	"github.com/joonnna/ifrit/client"
)

const (
	port = 5632
)

type bootStrapper struct {
	clientList      []*client.Client
	clientListMutex sync.RWMutex

	entryAddr string
}

func (b *bootStrapper) startClient() {
	b.clientListMutex.Lock()
	defer b.clientListMutex.Unlock()

	c, err := client.NewClient(b.entryAddr)
	if err != nil {
		fmt.Println(err)
		return
	}

	go c.Start()

	b.clientList = append(b.clientList, c)
}

func (b *bootStrapper) shutDownClients() {
	b.clientListMutex.RLock()
	defer b.clientListMutex.RUnlock()

	for _, c := range b.clientList {
		c.ShutDown()
	}
}

func (b *bootStrapper) addNodeHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	defer r.Body.Close()

	b.startClient()
}

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

	c, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}
	go c.Start(uint32(numRings))

	b := bootStrapper{
		clientList: make([]*client.Client, 0),
		entryAddr:  c.GetAddr(),
	}

	http.HandleFunc("/addNode", b.addNodeHandler)
	go http.ListenAndServe(fmt.Sprintf("129.242.80.135:%d", port), nil)

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	b.shutDownClients()
	c.Shutdown()

}
