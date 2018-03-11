package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/cauth"
)

var (
	KeyFilePath           = "ca_key.pem"
	CACertificateFilePath = "ca_cert.pem"
)

var (
	logger = log.New("module", "ca/main")
)

// saveState saves ca private key and public certificates to disk.
func saveState(ca *cauth.Ca) {

	f, err := os.Create(KeyFilePath)
	if err != nil {
		logger.Error("Cannot create CA private key file.", "file", KeyFilePath)
		panic(err)
	}

	logger.Info("Save CA private key.", "file", KeyFilePath)
	ca.SavePrivateKey(f)

	f, err = os.Create(CACertificateFilePath)
	if err != nil {
		logger.Error("Cannot create CA file.", "file", KeyFilePath)
		panic(err)
	}

	logger.Info("Save CA certificate.", "file", CACertificateFilePath)
	err = ca.SaveCertificate(f)

	if err != nil {
		panic(err)
	}
}

func main() {
	runtime.GOMAXPROCS(runtime.NumCPU())
	var numRings uint

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 10, "Number of gossip rings to be used")

	args.Parse(os.Args[1:])

	r := log.Root()

	h := log.CallerFileHandler(log.Must.FileHandler("calog", log.TerminalFormat()))

	r.SetHandler(h)

	ca, err := cauth.NewCa()
	if err != nil {
		panic(err)
	}

	saveState(ca)
	defer saveState(ca)

	go ca.Start(uint32(numRings))

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	ca.Shutdown()
}
