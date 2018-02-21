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

	"github.com/joonnna/ifrit"
	"github.com/joonnna/ifrit/bootstrap"
	"github.com/joonnna/ifrit/log"
)

var (
	clients []string
)

func createClients(requestChan chan interface{}, exitChan chan bool, arg string, vizAddr string) {
	test := true
	for {
		select {
		case <-requestChan:
			var addrs []string
			if len(clients) > 0 {
				idx := rand.Int() % len(clients)
				addrs = append(addrs, clients[idx])
				log.Info(addrs[0])
			}
			conf := &ifrit.Config{Visualizer: true, VisAddr: vizAddr, EntryAddrs: addrs}
			c, err := ifrit.NewClient(conf)
			if err != nil {
				fmt.Println(err)
				continue
			}

			clients = append(clients, c.Addr())
			c.RegisterMsgHandler(msgHandler)

			if test {
				activateSendTo(c)
				test = false
			}

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
			m := c.Members()
			if len(m) == 0 {
				continue
			}
			ch := c.SendTo(m[0], data)
			r := <-ch
			fmt.Println(string(r))
		}
	}()
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

	f, err := os.Create("/var/log/ifritlog")
	if err != nil {
		panic(err)
	}

	log.Init(f, log.DEBUG)

	l, err := bootstrap.NewLauncher(uint32(numRings), ch)
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
