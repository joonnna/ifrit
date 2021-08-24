package main

import (
	"errors"
	"time"
	"fmt"
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	_ "net/http/pprof"

	"github.com/joonnna/ifrit"

	log "github.com/inconshreveable/log15"
)

var (
	errNoAddr = errors.New("No certificate authority address provided, can't continue")
	logger    = log.New("module", "ifritclient/main")
)

func main() {
	var logfile string
	var h log.Handler

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&logfile, "logfile", "", "Log to file.")
	args.Parse(os.Args[1:])

	r := log.Root()

	if logfile != "" {
		h = log.CallerFileHandler(log.Must.FileHandler(logfile, log.LogfmtFormat()))
	} else {
		h = log.StreamHandler(os.Stdout, log.LogfmtFormat())
	}

	r.SetHandler(h)

	c, err := ifrit.NewClient(&ifrit.Config{
		New:      true,
		Hostname: "127.0.1.1",
		TCPPort:  2000,
		UDPPort:  3000,
		CryptoUnitPath: "crypto",
	})
	if err != nil {
		panic(err)
	}

	c.RegisterMsgHandler(msgHandler)
	go c.Start()
	

	for {
		if len(c.Members()) == 0 {
			continue
		}

		addr := c.Members()[0]
		ch := c.SendTo(addr, []byte("HellO!"))

		time.Sleep(3 * time.Second)
		select {
			case msg := <-ch:
			if msg != nil {
				fmt.Println("Got response from client:", msg)
			} else {
				break
			}
		}
	}
	
	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	if err := c.SavePrivateKey(); err != nil {
		panic(err)
	}

	if err := c.SaveCertificate(); err != nil {
		panic(err)
	}

	c.Stop()
}

func msgHandler(data []byte) ([]byte, error) {
	fmt.Println("Handler!:", string(data))

	return []byte("Return value from handler"), nil
}