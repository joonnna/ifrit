package netutils

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"strconv"
	"strings"
)

var (
	errFoundNoPort = errors.New("Couldnt find any available port")
)

func GetOpenPort() int {
	/*
		var addr string
		hostName, _ := os.Hostname()

		addrs, err := net.LookupHost(hostName)
		if err != nil {
			return 0
		}
		fmt.Println(addrs)

		if len(addrs) > 1 {
			addr = addrs[1]
		} else {
			addr = addrs[0]
		}
	*/
	attempts := 0
	for {
		//l, err := net.Listen("tcp4", fmt.Sprintf("%s:", addr))
		l, err := net.Listen("tcp4", ":0")
		if err == nil {
			addr := l.Addr().String()
			l.Close()
			tmp := strings.Split(addr, ":")
			port, _ := strconv.Atoi(tmp[len(tmp)-1])
			return port
		} else {
			fmt.Println(err)
		}
		attempts++
		if attempts > 100 {
			return 0
		}
	}

	return 0
}

func ListenOnPort(hostName string, port int) (net.Listener, error) {
	var l net.Listener
	var err error

	addrs, err := net.LookupHost(hostName)
	if err != nil {
		return l, err
	}

	startPort := rand.Int() % port

	for {
		l, err = net.Listen("tcp4", fmt.Sprintf("%s:%d", addrs[0], startPort))
		if err == nil {
			break
		}

		if startPort > (port + 100) {
			return l, errFoundNoPort
		}

		startPort++
	}

	return l, nil
}

func GetListener(hostName string) (net.Listener, error) {
	var l net.Listener
	var err error

	addrs, err := net.LookupHost(hostName)
	if err != nil {
		return l, err
	}

	attempts := 0

	for {
		l, err = net.Listen("tcp4", fmt.Sprintf("%s:", addrs[0]))
		if err == nil {
			return l, nil
		} else {
			fmt.Println(err)
		}
		attempts++

		if attempts > 100 {
			return l, errFoundNoPort
		}
	}

	return l, errFoundNoPort
}

//Hacky AF
func GetLocalIP() string {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}
