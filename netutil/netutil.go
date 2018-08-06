package netutil

import (
	"errors"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"

	log "github.com/inconshreveable/log15"
)

var (
	errFoundNoPort = errors.New("Couldnt find any available port")
	errNoAddr      = errors.New("Failed to find non-loopback address")
)

func GetOpenPort() int {
	attempts := 0
	for {
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

func ListenOnPort(port int) (net.Listener, error) {
	var l net.Listener
	var err error

	startPort := port
	h, _ := os.Hostname()

	addr, err := net.LookupHost(h)
	if err != nil {
		return nil, err
	}
	/*
		for _, a := range addr {
			log.Debug(a)
		}
	*/
	for {
		l, err = net.Listen("tcp4", fmt.Sprintf("%s:%d", addr[0], startPort))
		if err == nil {
			break
		}

		if startPort > (port + 100) {
			log.Error(err.Error())
			return l, errFoundNoPort
		}

		startPort++
	}

	return l, nil
}

func GetListener() (net.Listener, error) {
	var l net.Listener
	var err error

	attempts := 0

	h, _ := os.Hostname()

	addr, err := net.LookupHost(h)
	if err != nil {
		return nil, err
	}

	for {
		l, err = net.Listen("tcp4", fmt.Sprintf("%s:", addr[0]))
		if err == nil {
			return l, nil
		} else {
			log.Error(err.Error())
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
	/*
			conn, err := net.Dial("udp", "8.8.8.8:80")
			if err != nil {
				log.Error(err.Error())
			}
			defer conn.Close()

			localAddr := conn.LocalAddr().(*net.UDPAddr)

			return localAddr.IP.String()
		return h
	*/

	h, _ := os.Hostname()
	addr, err := net.LookupHost(h)
	if err != nil || len(addr) < 1 {
		return ""
	}

	return addr[0]
}

func LocalIP() (string, error) {
	host, _ := os.Hostname()

	addrs, err := net.LookupIP(host)
	if err != nil {
		return "", err
	}

	if len(addrs) == 0 {
		return "", errNoAddr
	}

	return addrs[0].String(), nil
}

func ListenUdp() (*net.UDPConn, string, error) {
	h, _ := os.Hostname()

	addr, err := net.LookupHost(h)
	if err != nil {
		return nil, "", err
	}

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:0", addr[0]))
	if err != nil {
		return nil, "", err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, "", err
	}

	port := strings.Split(conn.LocalAddr().String(), ":")[1]
	fullAddr := fmt.Sprintf("%s:%s", addr[0], port)

	return conn, fullAddr, nil
}
