package main

import (
	"fmt"
	"github.com/joonnna/capstone/client"
	"os"
	"os/signal"
	"syscall"
	"strings"
)

func main() {
	host, _ := os.Hostname()

	hostName := strings.Split(host, ".")[0]

	_ = client.StartClient("")

	entryPoint := fmt.Sprintf("%s:%d", hostName, 8345)

	for i := 0; i < 3; i++ {
		_ = client.StartClient(entryPoint)
	}

	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c
}
