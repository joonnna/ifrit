package netutils

import (
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

func GetLocalIP() string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return ""
	}

	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			return ""
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			return ip.String()
		}
	}
	return ""
}
