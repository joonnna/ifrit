package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	log "github.com/inconshreveable/log15"
	"github.com/jinzhu/configor"
	"github.com/joonnna/ifrit/cauth"
)

var Config = struct {
	Name         string `default:"Ifrit Test Groups"`
	Version      string `default:"1.0.0"`
	Host         string `default:""`
	Port         uint32 `default:"8080"`
	PathName     string `default:"./cadir"`
	NumRings     uint32 `default:"12"`
	NumBootNodes uint32 `default:"0"`
	LogFile      string `default:"./ifritd.log"`
}{}

// saveState saves ca private key and public certificates to disk.
func saveState(ca *cauth.Ca) {
	err := ca.SavePrivateKey()
	if err != nil {
		panic(err)
	}

	err = ca.SaveCertificate()
	if err != nil {
		panic(err)
	}
}

func main() {

	var h log.Handler

	runtime.GOMAXPROCS(runtime.NumCPU())

	configor.New(&configor.Config{Debug: true}).Load(&Config, "ca-config.yml")

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&Config.LogFile, "logfile", "", "Log to file.")
	args.Parse(os.Args[1:])

	r := log.Root()

	if logfile != "" {
		h = log.CallerFileHandler(log.Must.FileHandler(Config.LogFile, log.LogfmtFormat()))
	} else {
		h = log.StreamHandler(os.Stdout, log.LogfmtFormat())
	}

	r.SetHandler(h)

	ca, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}

	saveState(ca)
	defer saveState(ca)

	go ca.Start()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	ca.Shutdown()
}
