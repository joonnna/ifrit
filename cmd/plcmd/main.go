package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/joonnna/firechain/planetlab"
	"golang.org/x/crypto/ssh"
)

const (
	//clientPath = "/home/jon/Documents/Informatikk/golang/bin/firechainClient"
	clientPath = "/home/jon/Documents/Informatikk/golang/src/github.com/joonnna/firechain/cmd/firechainClient/firechainClient"
	caPath     = "/home/jon/Documents/Informatikk/golang/src/github.com/joonnna/firechain/cmd/ca/ca"
	addrPath   = "/home/jon/Documents/Informatikk/golang/src/github.com/joonnna/firechain/planetlab/node_addrs"
	clientCmd  = "./firechainClient -addr=195.113.161.14:35669"
	cleanCmd   = "pkill -9 firechainClient"
	aliveCmd   = "ps aux | grep -c firechainClient"
)

type cmdStatus struct {
	success bool
	addr    string
	result  string
}

func transferCmd(addrs []string, path string, conf *ssh.ClientConfig, c chan *cmdStatus) {
	for _, a := range addrs {
		go transferBinary(a, path, conf, c)
	}
}

func doCmds(addrs []string, mode string, cmd string, conf *ssh.ClientConfig, c chan *cmdStatus) {
	for _, a := range addrs {
		go doCmd(a, mode, cmd, conf, c)
	}
}

func transferBinary(addr string, path string, conf *ssh.ClientConfig, ch chan *cmdStatus) {
	success := true
	err := planetlab.TransferFile(addr, path, conf)
	if err != nil {
		log.Println(err)
		success = false
	}

	status := &cmdStatus{
		addr:    addr,
		success: success,
	}
	ch <- status
}

func doCmd(addr string, mode string, cmd string, conf *ssh.ClientConfig, ch chan *cmdStatus) {
	success := true
	out, err := planetlab.ExecuteCmd(addr, cmd, conf, mode)
	if err != nil {
		fmt.Println(err)
		success = false
	}

	status := &cmdStatus{
		addr:    addr,
		success: success,
		result:  out,
	}

	ch <- status
}

func main() {
	var fp string
	var cmd string
	var addrs []string
	var caAddr string

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&cmd, "cmd", "", "Command to execute")
	args.StringVar(&fp, "fp", addrPath, "Path to file containing node addresses")
	args.StringVar(&caAddr, "caAddr", "ple2.cesnet.cz", "Address to transfer ca binary")

	args.Parse(os.Args[1:])
	b, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Fatal(err)
	}

	if cmd == "transfer-ca" {
		addrs = append(addrs, caAddr)
	} else {
		lines := strings.Split(string(b[:]), "\n")

		for _, l := range lines {
			if strings.Contains(l, ",") {
				line := strings.Split(l, ",")
				if len(line) < 2 {
					continue
				}
				addrs = append(addrs, line[0])
			} else {
				addrs = append(addrs, l)
			}
			/*
				if len(addrs) >= 40 {
					break
				}
			*/
		}
	}
	fmt.Println("Nodes affected by cmd: ", len(addrs))

	conf, err := planetlab.GenSshConfig("")
	if err != nil {
		fmt.Println(err)
		return
	}

	c := make(chan *cmdStatus)

	switch cmd {
	case "transfer":
		transferCmd(addrs, clientPath, conf, c)
	case "transfer-ca":
		transferCmd(addrs, caPath, conf, c)
	case "clean":
		doCmds(addrs, "run", cleanCmd, conf, c)
	case "deploy":
		/*
			for i := 0; i < 2; i++ {
				addrs = append(addrs, addrs...)
			}
		*/
		doCmds(addrs, "start", clientCmd, conf, c)
	case "alive":
		doCmds(addrs, "run", aliveCmd, conf, c)
	default:
		fmt.Println("Command not supported")
		return
	}

	res, err := os.Create(cmd + "_res")
	if err != nil {
		log.Fatal(err)
	}
	defer res.Close()

	completed := 0
	failed := 0

	for i := 0; i < len(addrs); i++ {
		status := <-c
		if status.success {
			completed++
			fmt.Println("Success for : ", status.addr)
			fmt.Println("Completed: ", completed)
			fmt.Println(status.result)
			res.Write([]byte(status.addr + "\n"))
			res.Sync()
		} else {
			failed++
			fmt.Println("Failure for : ", status.addr)
			fmt.Println("Failed: ", failed)
		}
	}

	fmt.Printf("Completed %s cmd on %d nodes", cmd, completed)
}
