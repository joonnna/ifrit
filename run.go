package main

import (
	"fmt"
	"github.com/joonnna/capstone/client"
	"os"
	"os/signal"
	"syscall"
	"strings"
	"time"
)

func main() {
	host, _ := os.Hostname()

	hostName := strings.Split(host, ".")[0]

	var clientList []*client.Client

	c := client.StartClient("")

	clientList = append(clientList, c)

	entryPoint := fmt.Sprintf("%s:%d", hostName, 8345)

	for i := 0; i < 5; i++ {
		c = client.StartClient(entryPoint)
		clientList = append(clientList, c)
	}

	time.Sleep(30 * time.Second)

	clientList[0].ShutDownClient()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	for _, c := range clientList {
		c.ShutDownClient()
	}
}
