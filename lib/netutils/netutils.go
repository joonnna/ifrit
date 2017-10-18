package netutils

import (
	"log"
	"net"
	"strconv"
	"strings"
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
