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
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/joonnna/go-fireflies/planetlab"
	"golang.org/x/crypto/ssh"
)

const (
	caAddr     = "ple2.cesnet.cz"
	caPort     = 8090
	clientPort = 12300
	dosAddr    = "129.242.19.146:8100"
	dosIp      = "129.242.19.146"
)

type expArgs struct {
	numRings int
	byz      float32
	timeout  int
	addrs    []string
	maxConc  int
}

func sliceDiff(s1, s2 []string) []string {
	var retSlice []string
	m := make(map[string]bool)

	for _, s := range s1 {
		m[s] = true
	}

	for _, s := range s2 {
		if _, ok := m[s]; !ok {
			retSlice = append(retSlice, s)
		}
	}

	return retSlice
}

func deployLocal(caAddr string) error {
	cmd := exec.Command("/usr/local/go/bin/go", "run", "cmd/firefliesclient/main.go", fmt.Sprintf("-addr=%s:%d", caAddr, caPort))

	return cmd.Start()
}

func cleanLocal() error {
	cmd := exec.Command("pkill", "-9", "firefliesclient")
	return cmd.Run()
}

func startMeasurementRequest(addr string) error {
	var resp *http.Response
	var err error

	tries := 0
	startPort := clientPort

	for {
		url := fmt.Sprintf("http://%s:%d/startrecording", addr, startPort)
		resp, err = http.Get(url)
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

func startDosRequest(addr string, maxConc int) error {
	var resp *http.Response
	var err error

	r := bytes.NewReader([]byte(strconv.Itoa(maxConc)))

	tries := 0
	startPort := clientPort

	for {
		url := fmt.Sprintf("http://%s:%d/dos", addr, startPort)
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

func getDosResult(addr string, endPoint string) (int, error) {
	var resp *http.Response
	var err error

	url := fmt.Sprintf("http://%s:%d/%s", addr, 12300, endPoint)
	resp, err = http.Get(url)
	if err != nil {
		return 0, err
	}

	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	val, err := strconv.Atoi(string(bytes))
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	return val, nil
}

func getLatencies(addr string) ([]float64, error) {
	var ret []float64
	var resp *http.Response
	var err error

	tries := 0
	startPort := clientPort

	for {
		url := fmt.Sprintf("http://%s:%d/latencies", addr, startPort)
		resp, err = http.Get(url)
		if err == nil {
			break
		}

		if tries > 2 {
			return nil, err
		}
		startPort++
		tries++
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	content := string(bytes)

	values := strings.Split(content, "\n")

	for _, val := range values {
		f, err := strconv.ParseFloat(val, 64)
		if err != nil {
			fmt.Println(err)
			continue
		}

		ret = append(ret, f)
	}

	return ret, nil
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

		if tries > 2 {
			return 0, err
		}
		startPort++
		tries++
	}
	defer resp.Body.Close()
	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	val, err := strconv.Atoi(string(bytes))
	if err != nil {
		fmt.Println(err)
		return 0, err
	}

	return val, nil
}

func doDosExperiment(args *expArgs, conf, local *ssh.ClientConfig, res *os.File) {
	var deployed []string
	var numByz int

	saturationTimeout := 60
	expDuration := 60

	c := make(chan *planetlab.CmdStatus)

	cmd := fmt.Sprintf("%s -addr=%s:%d", planetlab.ClientCmd, caAddr, caPort)
	caCmd := fmt.Sprintf("%s -numRings=%d", planetlab.CaCmd, args.numRings)

	_, err := planetlab.ExecuteCmd(caAddr, caCmd, conf, planetlab.Start)
	if err != nil {
		fmt.Println(err)
		return
	}
	planetlab.DoCmds(args.addrs, planetlab.Start, cmd, conf, c)

	_, err = planetlab.ExecuteCmd(dosIp, cmd, local, planetlab.Start)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Deploying...")
	for i := 0; i < len(args.addrs); i++ {
		select {
		case s := <-c:
			if s.Success {
				deployed = append(deployed, s.Addr)
			}
		case <-time.After(time.Second * 30):
			break
		}
	}

	fmt.Printf("Deployed on %d nodes, waiting %d minutes for network saturation\n", len(deployed), saturationTimeout)

	time.Sleep(time.Minute * time.Duration(saturationTimeout))

	err = startMeasurementRequest(dosIp)
	if err != nil {
		fmt.Println(err)
		return
	}

	numByz = int(float32(len(deployed)) * args.byz)

	currByz := 0

	for _, addr := range deployed {
		if addr == dosIp {
			continue
		}
		err := startDosRequest(addr, args.maxConc)
		if err != nil {
			fmt.Println(err)
			continue
		}

		fmt.Println("DoS node: ", addr)
		currByz++
		if currByz >= numByz {
			break
		}
	}

	fmt.Printf("Started measurement on local node, waiting for experiment to complete(%d mins)\n", expDuration)

	time.Sleep(time.Minute * time.Duration(expDuration))

	fmt.Println("Cleaning ca and planetlab nodes")
	_, err = planetlab.ExecuteCmd(caAddr, planetlab.CaCleanCmd, conf, planetlab.Start)
	if err != nil {
		fmt.Println(err)
	}

	planetlab.DoCmds(deployed, planetlab.Start, planetlab.CleanCmd, conf, c)

	fmt.Println("Started result collection")

	completed, err := getDosResult(dosIp, "numrequests")
	if err != nil {
		fmt.Println("Failed to get results")
		fmt.Println(err)
	}

	failed, err := getDosResult(dosIp, "numfailedrequests")
	if err != nil {
		fmt.Println("Failed to get results")
		fmt.Println(err)
	}

	res.Write([]byte(fmt.Sprintf("nodeCount: %d\n", len(deployed))))
	res.Sync()

	res.Write([]byte(fmt.Sprintf("%d\t%d\t%f\t%d\t%d\n", args.numRings, args.maxConc, args.byz, completed, failed)))
	res.Sync()

	res.Write([]byte("\n"))
	res.Sync()

	fmt.Println("Finished, starting cleaning")

	_, err = planetlab.ExecuteCmd(dosIp, planetlab.CleanCmd, local, planetlab.Start)
	if err != nil {
		fmt.Println(err)
		return
	}

	time.Sleep(time.Minute * 1)
}

func doLatencyExperiment(args *expArgs, conf *ssh.ClientConfig, res *os.File) {
	var deployed, experimentNodes, byzNodes []string

	saturationTimeout := 5
	expDuration := 5

	c := make(chan *planetlab.CmdStatus)

	cmd := fmt.Sprintf("%s -addr=%s:%d", planetlab.ClientCmd, caAddr, caPort)
	caCmd := fmt.Sprintf("%s -numRings=%d", planetlab.CaCmd, args.numRings)

	_, err := planetlab.ExecuteCmd(caAddr, caCmd, conf, planetlab.Start)
	if err != nil {
		fmt.Println(err)
		return
	}

	planetlab.DoCmds(args.addrs, planetlab.Start, cmd, conf, c)

	fmt.Println("Deploying...")
	for i := 0; i < len(args.addrs); i++ {
		select {
		case s := <-c:
			if s.Success {
				deployed = append(deployed, s.Addr)
			}
		case <-time.After(time.Second * 30):
			break
		}
	}

	fmt.Printf("Deployed on %d nodes, waiting %d minutes for network saturation\n", len(deployed), saturationTimeout)

	time.Sleep(time.Minute * time.Duration(saturationTimeout))

	numByz := int(float32(len(deployed)) * args.byz)

	currByz := 0

	for _, addr := range deployed {
		err := startDosRequest(addr, args.timeout)
		if err != nil {
			fmt.Println(err)
			continue
		}

		byzNodes = append(byzNodes, addr)
		currByz++
		if currByz >= numByz {
			break
		}
	}

	correctNodes := sliceDiff(byzNodes, deployed)

	for _, addr := range correctNodes {
		err := startMeasurementRequest(addr)
		if err != nil {
			fmt.Println("Failed to start experiment on:", addr)
			fmt.Println(err)
			continue
		}

		experimentNodes = append(experimentNodes, addr)
	}

	fmt.Printf("Started measurement on nodes, waiting for experiment to complete %d minutes\n", expDuration)

	time.Sleep(time.Minute * time.Duration(expDuration))

	fmt.Println("Experiment done, cleaning corrupt nodes, 1 min sleep")

	planetlab.DoCmds(byzNodes, planetlab.Start, planetlab.CleanCmd, conf, c)

	time.Sleep(time.Minute * 1)

	fmt.Println("Started result collection")

	res.Write([]byte(fmt.Sprintf("nodeCount: %d\n", len(experimentNodes))))
	res.Sync()
	for _, addr := range experimentNodes {
		values, err := getLatencies(addr)
		if err != nil {
			fmt.Println("Failed to get results from: ", addr)
			fmt.Println(err)
			continue
		}

		completed, err := getDosResult(addr, "numrequests")
		if err != nil {
			fmt.Println("Failed to get results from: ", addr)
			fmt.Println(err)
		}
		failed, err := getDosResult(addr, "numfailedrequests")
		if err != nil {
			fmt.Println("Failed to get results from: ", addr)
			fmt.Println(err)
		}

		for _, lat := range values {
			res.Write([]byte(fmt.Sprintf("%s\t%d\t%d\t%f\t%f\t%d\t%d\n", addr, args.numRings, args.timeout, args.byz, lat, completed, failed)))
		}

		res.Sync()
	}
	res.Write([]byte("\n"))
	res.Sync()

	fmt.Println("Finished, starting cleaning")

	_, err = planetlab.ExecuteCmd(caAddr, planetlab.CaCleanCmd, conf, planetlab.Start)
	if err != nil {
		fmt.Println(err)
	}

	planetlab.DoCmds(deployed, planetlab.Start, planetlab.CleanCmd, conf, c)
	time.Sleep(time.Minute * 5)
	fmt.Println("Done cleaning")
}

func doFullExperiment(numrings int, addrs []string, conf *ssh.ClientConfig, res *os.File) {
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
	fmt.Printf("Deployed on %d nodes, waiting 1 hour for network saturation", len(deployed))

	time.Sleep(time.Minute * 60)

	for _, addr := range deployed {
		err := startMeasurementRequest(addr)
		if err != nil {
			fmt.Println("Failed to start experiment on:", addr)
			fmt.Println(err)
			continue
		}

		experimentNodes = append(experimentNodes, addr)
	}

	fmt.Println("Started measurement on nodes, waiting for experiment to complete(1 hour)")

	time.Sleep(time.Minute * 60)

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

		res.Write([]byte(fmt.Sprintf("%s\t%d\t%d\n", addr, numrings, val)))
		res.Sync()
	}
	res.Write([]byte("\n"))
	res.Sync()

	fmt.Println("Finished, starting cleaning")

	_, err = planetlab.ExecuteCmd(caAddr, planetlab.CaCleanCmd, conf, planetlab.Start)
	if err != nil {
		fmt.Println(err)
	}

	planetlab.DoCmds(deployed, planetlab.Start, planetlab.CleanCmd, conf, c)
	time.Sleep(time.Minute * 5)
	fmt.Println("Done cleaning")
}

func main() {
	var fp, cmd, resFile string
	var addrs []string
	var iterations int

	runtime.GOMAXPROCS(runtime.NumCPU())

	args := flag.NewFlagSet("args", flag.ExitOnError)
	args.StringVar(&cmd, "cmd", "", "Experiment to execute")
	args.StringVar(&fp, "fp", "planetlab/exp_nodes", "Path to file containing node addresses")
	args.StringVar(&resFile, "results", "exp_res", "Which file to store results")
	args.IntVar(&iterations, "iterations", 50, "Iterations for experiment")

	args.Parse(os.Args[1:])
	b, err := ioutil.ReadFile(fp)
	if err != nil {
		log.Fatal(err)
	}
	lines := strings.Split(string(b[:]), "\n")

	for _, l := range lines {
		if l != "" {
			addrs = append(addrs, l)
		}
	}

	conf, err := planetlab.GenSshConfig("uitple_firechain")
	if err != nil {
		fmt.Println(err)
		return
	}
	localConf, err := planetlab.GenSshConfig("jon")
	if err != nil {
		fmt.Println(err)
		return
	}

	res, err := os.Create(resFile)
	if err != nil {
		log.Fatal(err)
	}

	//rings := []int{10, 20, 40, 80, 160, 320, 640, 1280}
	byzPercent := []float32{0.05, 0.10, 0.15, 0.20, 0.25, 0.30, 0.35, 0.40}
	//timeouts := []int{5, 4, 3, 2, 1}

	for _, b := range byzPercent {
		args := &expArgs{
			maxConc:  10,
			byz:      b,
			numRings: 10,
			addrs:    addrs,
		}
		doDosExperiment(args, conf, localConf, res)
	}

}
