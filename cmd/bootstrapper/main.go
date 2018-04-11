package main

import (
	"flag"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	_ "net/http/pprof"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit"
	"github.com/joonnna/ifrit/bootstrap"
)

var (
	clients []string
)

func createClients(requestChan chan interface{}, exitChan chan bool, arg string, vizAddr string) {
	//test := true
	for {
		select {
		case <-requestChan:
			var addrs []string
			if len(clients) > 0 {
				idx := rand.Int() % len(clients)
				addrs = append(addrs, clients[idx])
				log.Info(addrs[0])
			}
			conf := &ifrit.Config{Visualizer: true, VisAddr: vizAddr, Ca: true, CaAddr: arg}
			c, err := ifrit.NewClient(conf)
			if err != nil {
				fmt.Println(err)
				continue
			}

			clients = append(clients, c.Addr())

			/*
				c.RegisterGossipHandler(gossipHandler)
				c.RegisterResponseHandler(gossipResponseHandler)
				c.SetGossipContent([]byte("This is a gossip message"))
					c.RegisterMsgHandler(msgHandler)

					activateSendTo(c)
			*/
			/*
				if test {
					activateSendTo(c)
					test = false
				}
			*/

			requestChan <- c
		case <-exitChan:
			return
		}

	}
}

func activateSendTo(c *ifrit.Client) {
	go func() {
		data := []byte("Application message boys!")
		for {
			time.Sleep(time.Second * 10)
			ch, num := c.SendToAll(data)
			for i := 0; i < num; i++ {
				r := <-ch
				fmt.Println(string(r))
			}
		}
	}()
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
	var numRings uint
	var vizAddr string

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 3, "Number of gossip rings to be used")
	args.StringVar(&vizAddr, "vizAddr", "127.0.0.1:8095", "Address of the visualizer (ip:port)")

	args.Parse(os.Args[1:])

	ch := make(chan interface{})
	exitChan := make(chan bool)

	r := log.Root()

	h := log.CallerFileHandler(log.Must.FileHandler("/var/log/ifritlog", log.TerminalFormat()))

	r.SetHandler(h)

	l, err := bootstrap.NewLauncher(uint32(numRings), ch, nil)
	if err != nil {
		panic(err)
	}

	go createClients(ch, exitChan, l.EntryAddr, vizAddr)
	go l.Start()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	l.ShutDown()
	close(exitChan)
}
