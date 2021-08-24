package main

import (
	"errors"
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
		New:      false,
		Hostname: "127.0.1.1",
		TCPPort:  2000,
		UDPPort:  3000,
		CryptoUnitPath: "crypto",
	})
	if err != nil {
		panic(err)
	}

	go c.Start()

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
