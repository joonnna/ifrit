package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"

	"github.com/joonnna/firechain/cauth"
	"github.com/joonnna/firechain/client"
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

	c := client.NewClient(b.entryAddr)

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
	fmt.Println("got req")
	b.startClient()
}

func main() {
	var numRings uint

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 3, "Number of gossip rings to be used")

	args.Parse(os.Args[1:])

	c, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}
	go c.Start(uint8(numRings))

	b := bootStrapper{
		clientList: make([]*client.Client, 0),
		entryAddr:  fmt.Sprintf("http://%s/certificateRequest", c.GetAddr()),
	}

	http.HandleFunc("/addNode", b.addNodeHandler)
	go http.ListenAndServe(fmt.Sprintf("localhost:%d", port), nil)

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	b.shutDownClients()
	c.Shutdown()
}
