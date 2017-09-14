package communication


import (
	"fmt"
	"os"
	"strings"
	"net"
	"errors"
	"sync"
	"github.com/joonnna/capstone/logger"
	"io"
	"bytes"
)

const (
	startPort = 8345
)

var (
	errReachable = errors.New("Remote entity not reachable")
)

type Comm struct {
	allConnections map[string]net.Conn
	connectionMutex sync.RWMutex

	localAddr string
	listener net.Listener

	log *logger.Log
}

func NewComm (log *logger.Log) *Comm {
	var l net.Listener
	var err error

	host, _ := os.Hostname()
	hostName := strings.Split(host, ".")[0]

	port := startPort

	for {
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%d", hostName, port))
		if err != nil {
			log.Err.Println(err)
		} else {
			break
		}
		port += 1
	}

	comm := &Comm{
		allConnections: make(map[string]net.Conn),
		localAddr: fmt.Sprintf("%s:%d", hostName, port),
		listener: l,
		log: log,
	}

	return comm
}

func (c *Comm) SendMsg (addr string, data []byte) error {
	return c.sendMsg(addr, data)
}

func (c *Comm) ReceiveMsg (cb func(reader io.Reader)) {
	for {
		conn, err := c.listener.Accept()
		if err != nil {
			c.log.Err.Println(err)
			return
		}

		go c.handleConn(conn, cb)
	}
}

func (c *Comm) BroadcastMsg (addrSlice []string, data []byte) {
	for _, addr := range addrSlice {
		_ = c.sendMsg(addr, data)
	}

}


func (c Comm) HostInfo () string {
	return c.localAddr
}


func (c *Comm) ShutDown () {
	c.listener.Close()
}


func (c *Comm) sendMsg(addr string, data []byte) error {
	var conn net.Conn
	var err error

	if !c.existConnection(addr) {
		conn, err = c.dial(addr)
		if err != nil {
			return errReachable
		}
	} else {
		conn = c.getConnection(addr)
	}

	_, err = conn.Write(data)
	if err != nil {
		c.log.Err.Println(err)
		conn, err = c.dial(addr)
		if err != nil {
			return errReachable
		}
	}
	return nil
}


func (c *Comm) dial (addr string) (net.Conn, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		c.log.Err.Println(err)
		return nil, errReachable
	} else {
		c.addConnection(addr, conn)
	}

	return conn, nil
}

func (c Comm) handleConn (conn net.Conn, cb func(reader io.Reader)) {
	data := make([]byte, 2048)

	_, err := conn.Read(data)
	if err != nil {
		c.log.Err.Println(err)
		return
	} else {
		cb(bytes.NewReader(data))
	}
}
