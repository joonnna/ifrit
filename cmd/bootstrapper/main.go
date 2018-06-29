package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	_ "net/http/pprof"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit"
	"github.com/joonnna/ifrit/bootstrap"
)

var (
	clients []string
)

func createClients(requestChan chan interface{}, exitChan chan bool) {
	for {
		select {
		case <-requestChan:
			var addrs []string
			if len(clients) > 0 {
				idx := rand.Int() % len(clients)
				addrs = append(addrs, clients[idx])
				log.Info(addrs[0])
			}
			c, err := ifrit.NewClient()
			if err != nil {
				log.Error(err.Error())
				continue
			}

			clients = append(clients, c.Addr())

			requestChan <- c
		case <-exitChan:
			return
		}
	}
}

func gossipHandler(data []byte) ([]byte, error) {
	fmt.Println(string(data))

	resp := []byte("This is a gossip response!!!")

	return resp, nil
}

func gossipResponseHandler(data []byte) {
	fmt.Println(string(data))
}

func msgHandler(data []byte) ([]byte, error) {
	fmt.Println(string(data))

	resp := []byte("Got message, here is response!!!")

	return resp, nil
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())

	exitChan := make(chan bool)

	r := log.Root()

	h := log.CallerFileHandler(log.Must.FileHandler("/var/log/ifritlog", log.TerminalFormat()))

	r.SetHandler(h)

	ch := make(chan interface{})
	l, err := bootstrap.NewLauncher(ch, nil)
	if err != nil {
		panic(err)
	}

	go l.Start()
	go createClients(ch, exitChan)

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	l.ShutDown()
	close(exitChan)
}
