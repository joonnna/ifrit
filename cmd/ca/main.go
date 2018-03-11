package main

import (
	"flag"
	"os"
	"os/signal"
	"runtime"
	"syscall"

	"github.com/joonnna/ifrit/cauth"
)

// saveState saves ca private key and public certificates to disk.
func saveState(ca *cauth.Ca) {

	f, err := os.Create("./caKey.pem")
	if err != nil {
		panic(err)
	}
	ca.SavePrivateKey(f)

	f, err = os.Create("./caCerts.pem")
	if err != nil {
		panic(err)
	}

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
