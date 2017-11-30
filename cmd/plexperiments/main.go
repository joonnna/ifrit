package main

import (
	"bytes"
	"encoding/json"
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
	dosAddr    = "129.242.19.134:8100"
	dosIp      = "129.242.19.134"
)

type expArgs struct {
	numRings int
	byz      float32
	timeout  int
	maxConc  int
	addrs    []string
}

func isNeighbor(addrs []string, addr string) bool {
	for _, a := range addrs {
		if a == addr {
			return true
		}
	}

	return false
}

func getNeighbors(ip string) ([]string, error) {
	var ret []string
	url := fmt.Sprintf("http://%s:%d/neighbors", ip, clientPort)
	resp, err := http.Get(url)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	resp.Body.Close()

	addrs := strings.Split(string(bytes), "\n")

	for _, a := range addrs {
		if a == "" {
			continue
		}
		ret = append(ret, strings.Split(a, ":")[0])
	}

	return ret, nil
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

func startMeasurementRequest(addr string, r io.Reader) error {
	var resp *http.Response
	var err error

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

func startDosRequest(addr string, body io.Reader) error {
	url := fmt.Sprintf("http://%s:%d/dos", addr, clientPort)
	resp, err := http.Post(url, "text", body)
	if err != nil {
		return err
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

func doDosExperiment(args *expArgs, conf, local *ssh.ClientConfig) {
	var deployed []string
	var numByz int

	saturationTimeout := 30
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

	measureArgs := struct {
		Byz      float32
		NumRings int
		MaxConc  int
		Duration int
		Timeout  int
	}{
		Byz:      args.byz,
		NumRings: args.numRings,
		MaxConc:  args.maxConc,
		Duration: expDuration,
		Timeout:  args.timeout,
	}

	b := new(bytes.Buffer)
	json.NewEncoder(b).Encode(measureArgs)

	neighbors, err := getNeighbors(dosIp)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Println("Neighbors: ", neighbors)

	err = startMeasurementRequest(dosIp, b)
	if err != nil {
		fmt.Println(err)
		return
	}

	numByz = int(float32(len(deployed)) * args.byz)

	currByz := 0

	dosArgs := struct {
		Addr    string
		Conc    int
		Timeout int
	}{
		Addr:    dosAddr,
		Conc:    args.maxConc,
		Timeout: args.timeout,
	}

	dosBytes := new(bytes.Buffer)
	json.NewEncoder(dosBytes).Encode(dosArgs)

	for _, addr := range deployed {
		if addr == dosIp || isNeighbor(neighbors, addr) {
			continue
		}

		err := startDosRequest(addr, dosBytes)
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

	time.Sleep(time.Minute * time.Duration(expDuration+10))

	fmt.Println("Finished, starting cleaning")
	_, err = planetlab.ExecuteCmd(caAddr, planetlab.CaCleanCmd, conf, planetlab.Start)
	if err != nil {
		fmt.Println(err)
	}

	planetlab.DoCmds(deployed, planetlab.Start, planetlab.CleanCmd, conf, c)

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
		if currByz >= numByz {
			break
		}
		/*
			err := startDosRequest(addr, args.timeout)
			if err != nil {
				fmt.Println(err)
				continue
			}
		*/

		byzNodes = append(byzNodes, addr)
		currByz++
	}

	correctNodes := sliceDiff(byzNodes, deployed)

	for _, _ = range correctNodes {
		/*
			err := startMeasurementRequest(addr)
			if err != nil {
				fmt.Println("Failed to start experiment on:", addr)
				fmt.Println(err)
				continue
			}

			experimentNodes = append(experimentNodes, addr)
		*/
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

	for _, _ = range deployed {
		/*
			err := startMeasurementRequest(addr)
			if err != nil {
				fmt.Println("Failed to start experiment on:", addr)
				fmt.Println(err)
				continue
			}

			experimentNodes = append(experimentNodes, addr)
		*/
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
	/*
		res, err := os.Create(resFile)
		if err != nil {
			log.Fatal(err)
		}
	*/

	//rings := []int{10, 20, 40, 80, 160, 320, 640, 1280}
	byzPercent := []float32{0.05, 0.10, 0.15, 0.20, 0.25, 0.30, 0.35, 0.40}
	//byzPercent := []float32{0.50}
	conc := []int{100, 200, 400, 800}
	//conc := []int{100}
	//timeouts := []int{5, 4, 3, 2, 1}

	for _, b := range byzPercent {
		for _, c := range conc {
			args := &expArgs{
				maxConc:  c,
				byz:      b,
				numRings: 1,
				addrs:    addrs,
				timeout:  1,
			}
			doDosExperiment(args, conf, localConf)
		}
	}

}
