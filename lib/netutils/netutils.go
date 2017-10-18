package netutils

import (
	"errors"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
)

var (
	errFoundNoPort = errors.New("Couldnt find any available port")
)

func GetOpenPort() int {
	for {
		l, err := net.Listen("tcp", ":0")
		if err == nil {
			addr := l.Addr().String()
			l.Close()
			tmp := strings.Split(addr, ":")
			port, _ := strconv.Atoi(tmp[len(tmp)-1])
			return port
		}
	}

	return 0
}

func GetListener(hostName string) (net.Listener, error) {
	var l net.Listener
	var err error

	for {
		l, err = net.Listen("tcp", fmt.Sprintf("%s:0", hostName))
		if err == nil {
			return l, nil
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
