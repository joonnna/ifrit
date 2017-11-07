package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joonnna/firechain/planetlab"
	"golang.org/x/crypto/ssh"
)

const (
	caPort     = 8090
	clientPort = 12300
)

func startMeasurementRequest(addr string) error {
	var resp *http.Response
	var err error

	r := bytes.NewReader([]byte("1"))

	tries := 0
	startPort := clientPort

	for {
		url := fmt.Sprintf("http://%s:%d/startrecording", addr, startPort)
		resp, err = http.Post(url, "text", r)
		if err == nil {
			break
		}

		if tries > 2 {
			return err
		}
		startPort++
		tries++
	}
	io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close()

	return nil
}

func getResult(addr string) (int, error) {
	var resp *http.Response
	var err error

	tries := 0
	startPort := clientPort

	for {
		url := fmt.Sprintf("http://%s:%d/numrequests", addr, startPort)
		resp, err = http.Get(url)
		if err == nil {
			break
		}

		if tries > 10 {
			return 0, err
		}
		startPort++
		tries++
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}
	defer resp.Body.Close()

	val, err := strconv.Atoi(string(bytes))
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	return val, nil
}

func doExperiment(caAddr string, numrings, iteration int, addrs []string, conf *ssh.ClientConfig, res *os.File) {
	var deployed, experimentNodes []string
	c := make(chan *planetlab.CmdStatus)

	cmd := fmt.Sprintf("%s -addr=%s:%d", planetlab.ClientCmd, caAddr, caPort)
	caCmd := fmt.Sprintf("%s -numRings=%d", planetlab.CaCmd, numrings)

	_, err := planetlab.ExecuteCmd(caAddr, caCmd, conf, planetlab.Start)
	if err != nil {
		fmt.Println(err)
		return
	}

	planetlab.DoCmds(addrs, planetlab.Start, cmd, conf, c)

	fmt.Println("Deploying...")
	for i := 0; i < len(addrs); i++ {
		select {
		case s := <-c:
			if s.Success {
				deployed = append(deployed, s.Addr)
			}
		case <-time.After(time.Second * 30):
			break
		}
	}
	fmt.Printf("Deployed on %d nodes, waiting 10 mins for network saturation", len(deployed))

	time.Sleep(time.Second * 30)

	for _, addr := range deployed {
		err := startMeasurementRequest(addr)
		if err != nil {
			fmt.Println("Failed to start experiment on:", addr)
			fmt.Println(err)
			continue
		}

		experimentNodes = append(experimentNodes, addr)
	}

	fmt.Println("Started measurement on nodes, waiting for experiment to complete(10min)")

	time.Sleep(time.Second * 30)

	fmt.Println("Started result collection")

	res.Write([]byte(fmt.Sprintf("nodeCount: %d\n", len(experimentNodes))))
	res.Sync()
	for _, addr := range experimentNodes {
		val, err := getResult(addr)
		if err != nil {
			fmt.Println("Failed to get results from: ", addr)
			fmt.Println(err)
			continue
		}

		res.Write([]byte(fmt.Sprintf("%s\t%d\t%d\t%d\n", addr, numrings, val, iteration)))
		res.Sync()
	}
	res.Write([]byte("\n"))
	res.Sync()

	fmt.Printf("Finished iteration %d, starting teardown", iteration)

	_, err = planetlab.ExecuteCmd(caAddr, planetlab.CaCleanCmd, conf, planetlab.Start)
	if err != nil {
		fmt.Println(err)
	}

	planetlab.DoCmds(deployed, planetlab.Start, planetlab.CleanCmd, conf, c)
	time.Sleep(time.Minute * 1)
	fmt.Println("Done cleaning")
	/*

		ch := make(chan *planetlab.CmdStatus)
		for i := 0; i < len(deployed); i++ {
			select {
			case s := <-c:
				if s.Success {
					cleaned = append(cleaned, s.Addr)
				}
				fmt.Println(len(cleaned))
			case <-time.After(time.Second * 30):
				fmt.Println(len(cleaned))
				break
			}
		}
		fmt.Printf("cleaned %d nodes", len(cleaned))
	*/
}

func main() {
	var fp, cmd, caAddr, resFile string
	var addrs []string
	var iterations int

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&cmd, "cmd", "", "Experiment to execute")
	args.StringVar(&fp, "fp", planetlab.AddrPath, "Path to file containing node addresses")
	args.StringVar(&caAddr, "caAddr", "ple2.cesnet.cz", "Address to deploy ca on")
	args.StringVar(&resFile, "results", "exp_res", "Which file to store results")
	args.IntVar(&iterations, "iterations", 100, "How many iterations of the experiment to run")

	args.Parse(os.Args[1:])
	b, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(b[:]), "\n")

	for _, l := range lines {
		addrs = append(addrs, l)
	}

	conf, err := planetlab.GenSshConfig()
	if err != nil {
		fmt.Println(err)
		return
	}

	res, err := os.Create(resFile)
	if err != nil {
		log.Fatal(err)
	}

	rings := []int{10, 50, 100, 1000}

	for _, numRings := range rings {
		for i := 0; i < iterations; i++ {
			doExperiment(caAddr, numRings, i, addrs, conf, res)
		}
	}
}
