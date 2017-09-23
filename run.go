package main

import (
	"os"
	"os/signal"
	"syscall"

	"github.com/joonnna/capstone/cauth"
	"github.com/joonnna/capstone/client"
)

func startCa() string {
	addrChan := make(chan string)

	c, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}

	err = c.NewGroup()
	if err != nil {
		panic(err)
	}

	go c.HttpHandler(addrChan)

	return <-addrChan
}

func main() {
	caAddr := startCa()
	/*
		host, _ := os.Hostname()
		hostName := strings.Split(host, ".")[0]
	*/
	var clientList []*client.Client

	c := client.StartClient(caAddr)

	clientList = append(clientList, c)

	for i := 0; i < 8; i++ {
		c = client.StartClient(caAddr)
		clientList = append(clientList, c)
	}

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	for _, c := range clientList {
		c.ShutDownClient()
	}
}
