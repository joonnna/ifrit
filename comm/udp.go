package comm

import (
	"net"
	"time"

	"github.com/golang/protobuf/proto"
	log "github.com/inconshreveable/log15"
	pb "github.com/joonnna/ifrit/protobuf"
)

type UDPServer struct {
	conn *net.UDPConn
	addr string

	exitChan  chan bool
	pauseChan chan time.Duration

	pongSigner
}

type pongSigner interface {
	Sign([]byte) ([]byte, []byte, error)
}

func NewUdpServer(ps pongSigner, conn *net.UDPConn) (*UDPServer, error) {
	return &UDPServer{
		conn:       conn,
		exitChan:   make(chan bool, 1),
		pauseChan:  make(chan time.Duration, 1),
		pongSigner: ps,
	}, nil
}

func (us *UDPServer) Ping(addr string, p *pb.Ping) (*pb.Pong, error) {
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

	data, err := proto.Marshal(p)
	if err != nil {
		return nil, err
	}

	_, err = c.Write(data)
	if err != nil {
		return nil, err
	}

	bytes := make([]byte, 256)

	n, err := c.Read(bytes)
	if err != nil {
		return nil, err
	}

	pong := &pb.Pong{}

	err = proto.Unmarshal(bytes[:n], pong)
	if err != nil {
		return nil, err
	}

	return pong, nil
}

func (us *UDPServer) Start() {
	bytes := make([]byte, 256)
	for {
		select {
		case d := <-us.pauseChan:
			time.Sleep(d)
		case <-us.exitChan:
			return
		default:
			n, addr, err := us.conn.ReadFrom(bytes)
			if err != nil {
				log.Error(err.Error())
				continue
			}

			r, s, err := us.Sign(bytes[:n])
			if err != nil {
				log.Error(err.Error())
				continue
			}

			pong := &pb.Pong{
				Signature: &pb.Signature{
					R: r,
					S: s,
				},
			}

			resp, err := proto.Marshal(pong)
			if err != nil {
				log.Error(err.Error())
				continue
			}

			us.conn.SetWriteDeadline(time.Now().Add(time.Second * 3))
			_, err = us.conn.WriteTo(resp, addr)
			if err != nil {
				log.Error(err.Error())
				continue
			}
		}
	}
}

func (us *UDPServer) Addr() string {
	return us.addr
}

func (us *UDPServer) Pause(d time.Duration) {
	us.pauseChan <- d
}

func (us *UDPServer) Stop() {
	close(us.exitChan)
	us.conn.Close()
}
