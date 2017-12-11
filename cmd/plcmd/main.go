package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"runtime"
	"strings"

	"github.com/joonnna/ifrit/planetlab"
)

const (
	caPort = 8090
	//localIp = "129.242.19.134"
	localIp = "129.242.15.67"
)

func main() {
	var fp string
	var cmd string
	var addrs []string
	var caAddr string

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&cmd, "cmd", "", "Command to execute")
	args.StringVar(&fp, "fp", planetlab.AddrPath, "Path to file containing node addresses")
	args.StringVar(&caAddr, "caAddr", "195.113.161.14", "Address to transfer ca binary")

	args.Parse(os.Args[1:])
	b, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Fatal(err)
	}

	if cmd == "transfer-ca" {
		addrs = append(addrs, caAddr)
	} else if cmd == "transfer-local" {
		addrs = append(addrs, localIp)
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

	conf, err := planetlab.GenSshConfig("uitple_firechain")
	if err != nil {
		fmt.Println(err)
		return
	}

	c := make(chan *planetlab.CmdStatus)

	switch cmd {
	case "transfer":
		planetlab.TransferToHosts(addrs, planetlab.ClientPath, conf, c)
	case "transfer-ca":
		planetlab.TransferToHosts(addrs, planetlab.CaPath, conf, c)
	case "clean":
		_, err := planetlab.ExecuteCmd(caAddr, planetlab.CaCleanCmd, conf, planetlab.Run)
		if err != nil {
			log.Println(err)
		}
		planetlab.DoCmds(addrs, planetlab.Run, planetlab.CleanCmd, conf, c)
	case "deploy":
		_, err := planetlab.ExecuteCmd(caAddr, planetlab.CaCmd, conf, planetlab.Start)
		if err != nil {
			log.Fatal(err)
		}
		cmd := fmt.Sprintf("%s -addr=%s:%d", planetlab.ClientCmd, caAddr, caPort)
		planetlab.DoCmds(addrs, planetlab.Start, cmd, conf, c)
	case "alive":
		planetlab.DoCmds(addrs, planetlab.Run, planetlab.AliveCmd, conf, c)
	case "transfer-local":
		local, err := planetlab.GenSshConfig("jon")
		if err != nil {
			fmt.Println(err)
			return
		}
		planetlab.TransferToHosts(addrs, planetlab.ClientPath, local, c)
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
		if status.Success {
			completed++
			fmt.Println("Success for : ", status.Addr)
			fmt.Println("Completed: ", completed)
			fmt.Println(status.Result)
			res.Write([]byte(status.Addr + "\n"))
			res.Sync()
		} else {
			failed++
			fmt.Println("Failure for : ", status.Addr)
			fmt.Println("Failed: ", failed)
		}
	}

	fmt.Printf("Completed %s cmd on %d nodes", cmd, completed)
}
