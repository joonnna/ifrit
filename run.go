package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/joonnna/capstone/cauth"
	"github.com/joonnna/capstone/client"
)

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

func main() {
	caAddr := startCa()
	fmt.Println(caAddr)
	/*
		host, _ := os.Hostname()
		hostName := strings.Split(host, ".")[0]
	*/

	entry := fmt.Sprintf("%s/certificateRequest", caAddr)

	var clientList []*client.Client
	c := client.StartClient(entry)

	clientList = append(clientList, c)

	for i := 0; i < 7; i++ {
		c = client.StartClient(entry)
		clientList = append(clientList, c)
	}

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	for _, c := range clientList {
		c.ShutDownClient()
	}
}
