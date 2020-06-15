package main

import (
	"flag"
	"fmt"
	"github.com/jinzhu/configor"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/cauth"
)

// Config contains all configurable parameters for the Ifrit CA daemon.
var Config = struct {
	Name            string `default:"TestGroup"`
	Version         string `default:"1.0.0"`
	Host            string `default:"127.0.0.1"`
	Port            int    `default:"8321"`
	KeyPath         string `default:"./key.pem"`
	CertificatePath string `default:"./cert.pem"`
	NumRings        uint32 `default:"3"`
	NumBootNodes    uint32 `default:"0"`
	LogFile         string `default:""`
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
	var configFile string

	runtime.GOMAXPROCS(runtime.NumCPU())

	// Load configuration from files and ENV
	configor.New(&configor.Config{Debug: true, ENVPrefix: "IFRIT"}).Load(&Config, configFile, "/etc/ifrit/config.yaml")

	// Override configuration with parameters
	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&configFile, "config", "./ca_config.yaml", "Configuration file.")
	args.StringVar(&Config.LogFile, "logfile", "", "Log to file.")

	args.StringVar(&Config.Host, "host", Config.Host, "Hostname")
	args.IntVar(&Config.Port, "port", Config.Port, "Port")

	args.Parse(os.Args[1:])

	r := log.Root()

	if Config.LogFile != "" {
		h = log.CallerFileHandler(log.Must.FileHandler(Config.LogFile, log.LogfmtFormat()))
	} else {
		h = log.StreamHandler(os.Stdout, log.LogfmtFormat())
	}

	r.SetHandler(h)

	fmt.Printf("Starting Ifrit CAd on port %d\n", Config.Port)

	ca, err := cauth.NewCa(Config.Host, Config.Port, Config.KeyPath, Config.CertificatePath, Config.NumRings, Config.NumBootNodes)
	if err != nil {
		panic(err)
	}

	//saveState(ca)
	//defer saveState(ca)

	go ca.Start()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	ca.Shutdown()
}
