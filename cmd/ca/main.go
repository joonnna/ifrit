package main

import (
	"flag"
	"os"

	"github.com/joonnna/capstone/cauth"
)

func main() {
	var numRings uint8

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.UintVar(&numRings, "numRings", 3, "Number of gossip rings to be used")

	args.Parse(os.Args[1:])

	ca := cauth.NewCa(caAddr)
	if err != nil {
		panic(err)
	}

	ca.Start(numRings)
}
