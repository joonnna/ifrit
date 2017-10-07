package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/joonnna/capstone/cauth"
	"github.com/joonnna/capstone/client"
)

const (
	port = 5632
)

type bootStrapper struct {
	clientList      []*client.Client
	clientListMutex sync.RWMutex

	entryAddr string
}

func startCa() string {
	var i uint8
	i = 3
	addrChan := make(chan string)

	c, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}

	err = c.NewGroup(i)
	if err != nil {
		panic(err)
	}

	go c.HttpHandler(addrChan)

	return <-addrChan
}

func (b *bootStrapper) startClient() {
	b.clientListMutex.Lock()
	defer b.clientListMutex.Unlock()

	b.clientList = append(b.clientList, client.StartClient(b.entryAddr))
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
	b := bootStrapper{
		clientList: make([]*client.Client, 0),
		entryAddr:  fmt.Sprintf("%s/certificateRequest", startCa()),
	}

	http.HandleFunc("/addNode", b.addNodeHandler)
	go http.ListenAndServe(fmt.Sprintf(":%d", port), nil)

	/*
		for i := 0; i < 7; i++ {
			b.startClient()
		}
	*/
	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	b.shutDownClients()
}
