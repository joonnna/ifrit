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

var (
	logger = log.New("module", "ca/main")
)

// saveState saves ca private key and public certificates to disk.
func saveState(ca *cauth.Ca) {

	f, err := os.Create(CAConfig.KeyFile)
	if err != nil {
		logger.Error("Cannot create CA private key file.", "file", CAConfig.KeyFile)
		panic(err)
	}

	logger.Info("Save CA private key.", "file", CAConfig.KeyFile)
	ca.SavePrivateKey(f)

	f, err = os.Create(CAConfig.CertificateFile)
	if err != nil {
		logger.Error("Cannot create CA file.", "file", CAConfig.CertificateFile)
		panic(err)
	}

	logger.Info("Save CA certificate.", "file", CAConfig.CertificateFile)
	err = ca.SaveCertificate(f)

	if err != nil {
		panic(err)
	}
}

func main() {

	runtime.GOMAXPROCS(runtime.NumCPU())

	configor.New(&configor.Config{ENVPrefix: "CA"}).Load(&CAConfig, "/etc/ifrit/ca/config.yml", "config.yml")

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&CAConfig.NumRings, "numRings", CAConfig.NumRings, "Number of gossip rings to be used")
	args.StringVar(&CAConfig.LogFile, "logfile", "", "Log to file.")

	args.Parse(os.Args[1:])

	r := log.Root()

	if CAConfig.LogFile != "" {
		h := log.CallerFileHandler(log.Must.FileHandler(CAConfig.LogFile, log.LogfmtFormat()))
		r.SetHandler(h)
	} else {
		h := log.CallerFileHandler(log.Must.FileHandler("calog", log.TerminalFormat()))
		r.SetHandler(h)
	}

	ca, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}

	saveState(ca)
	defer saveState(ca)

	go ca.Start(uint32(CAConfig.NumRings))

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	ca.Shutdown()
}
