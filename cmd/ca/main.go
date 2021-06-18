package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"

	"github.com/jinzhu/configor"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/cauth"
)

var DefaultPermission = os.FileMode(0750)

// Config contains all configurable parameters for the Ifrit CA daemon.
var Config = struct {
	Name         string `default:"Ifrit Certificate Authority"`
	Version      string `default:"1.0.0"`
	Host         string `default:"127.0.1.1"`
	Port         int    `default:"8321"`
	Path         string `default:"./ifrit-cad"`
	NumRings     uint32 `default:"3"`
	NumBootNodes uint32 `default:"5"`
	LogFile      string `default:""`
}{}

// saveState saves ca private key and public certificates to disk.
func saveState(ca *cauth.Ca) {

	log.Info("Saving CA state")

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
	var createNew bool

	runtime.GOMAXPROCS(runtime.NumCPU())

	// Override configuration with parameters
	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&configFile, "config", "./ca_config.yaml", "Configuration file.")

	// Load configuration from files and ENV
	configor.New(&configor.Config{Debug: false, ENVPrefix: "IFRIT"}).Load(&Config, configFile, "/etc/ifrit/config.yaml")

	args.StringVar(&Config.LogFile, "logfile", "", "Log to file.")
	args.StringVar(&Config.Host, "host", Config.Host, "Hostname.")
	args.IntVar(&Config.Port, "port", Config.Port, "Port number.")
	args.StringVar(&Config.Path, "path", Config.Path, "Path to runtime files.")
	args.BoolVar(&createNew, "new", false, "Initialize new CA structure.")
	args.Parse(os.Args[1:])

	// Setup logging
	r := log.Root()
	if Config.LogFile != "" {
		h = log.CallerFileHandler(log.Must.FileHandler(Config.LogFile, log.LogfmtFormat()))
	} else {
		h = log.StreamHandler(os.Stdout, log.LogfmtFormat())
	}

	r.SetHandler(h)

	// Attempt to load existing group
	var ca *cauth.Ca
	var err error

	if !createNew {
		ca, err = cauth.LoadCa(Config.Path, Config.NumRings, Config.NumBootNodes)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Error loading CA. Run with --new option if CA does not exit.")
			os.Exit(1)
		}
	} else {
		fmt.Println("Config:", Config)

		// Create run directory
		err := os.MkdirAll(Config.Path, DefaultPermission)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		ca, err = cauth.NewCa(Config.Path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}

		// Add initial group.
		err = ca.NewGroup(Config.NumRings, Config.NumBootNodes)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}

	// Save state
	saveState(ca)
	defer saveState(ca)

	// Start the daemon
	fmt.Printf("Starting Ifrit CA daemon on port %d\n", Config.Port)
	go ca.Start(Config.Host, strconv.Itoa(Config.Port))

	// Handle SIGTERM
	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	ca.Shutdown()
}
