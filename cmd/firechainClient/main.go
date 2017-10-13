package main

import (
	"flag"
	"os"

	"github.com/joonnna/firechain/client"
)

func main() {
	var caAddr string

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&caAddr, "addr", "", "address(ip:port) of certificate authority")

	args.Parse(os.Args[1:])

	c := client.NewClient(caAddr)

	c.Start()
}
