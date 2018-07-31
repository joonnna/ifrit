package udp

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	log "github.com/inconshreveable/log15"
)

type Server struct {
	conn *net.UDPConn
	addr string

	exitChan  chan bool
	pauseChan chan time.Duration
}

func NewServer() (*Server, error) {
	h, _ := os.Hostname()

	addr, err := net.LookupHost(h)
	if err != nil {
		return nil, err
	}

	udpAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:0", addr[0]))
	if err != nil {
		return nil, err
	}

	conn, err := net.ListenUDP("udp", udpAddr)
	if err != nil {
		return nil, err
	}

	port := strings.Split(conn.LocalAddr().String(), ":")[1]

	return &Server{
		conn:      conn,
		addr:      fmt.Sprintf("%s:%s", addr[0], port),
		exitChan:  make(chan bool, 1),
		pauseChan: make(chan time.Duration, 1),
	}, nil
}

func (s *Server) Send(addr string, data []byte) ([]byte, error) {
	udpAddr, err := net.ResolveUDPAddr("udp", addr)
	if err != nil {
		return nil, err
	}

	c, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return nil, err
	}
	defer c.Close()
	c.SetDeadline(time.Now().Add(time.Second * 5))

	_, err = c.Write(data)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, 256)

	n, err := c.Read(bytes)
	if err != nil {
		return nil, err
	}

	return bytes[:n], nil
}

func (s *Server) Serve(signMsg func([]byte) ([]byte, error)) error {
	bytes := make([]byte, 256)
	for {
		select {
		case d := <-s.pauseChan:
			time.Sleep(d)
		case <-s.exitChan:
			return nil
		default:
			n, addr, err := s.conn.ReadFrom(bytes)
			if err != nil {
				log.Error(err.Error())
				continue
			}

			resp, err := signMsg(bytes[:n])
			if err != nil {
				log.Error(err.Error())
				continue

			}

			s.conn.SetWriteDeadline(time.Now().Add(time.Second * 3))
			_, err = s.conn.WriteTo(resp, addr)
			if err != nil {
				log.Error(err.Error())
				continue
			}
		}
	}

	return nil
}

func (s *Server) Addr() string {
	return s.addr
}

func (s *Server) Pause(d time.Duration) {
	s.pauseChan <- d
}

func (s *Server) Stop() {
	close(s.exitChan)
	s.conn.Close()
}
