package core

import (
	"crypto/rand"
	"errors"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/joonnna/ifrit/core/discovery"
	pb "github.com/joonnna/ifrit/protobuf"
)

var (
	errDead                 = errors.New("Peer is dead")
	errInvalidPongSignature = errors.New("Invalid signature on pong message")
)

type failureDetector struct {
	ps             pingService
	cs             cryptoService
	maxFailedPings uint32
}

type pingService interface {
	Pause(time.Duration)
	Ping(string, *pb.Ping) (*pb.Pong, error)
	Start()
	Stop()
}

func newFd(ps pingService, cs cryptoService, maxPing uint32) *failureDetector {
	return &failureDetector{
		ps:             ps,
		cs:             cs,
		maxFailedPings: maxPing,
	}
}

func (fd *failureDetector) stopServing(d time.Duration) {
	fd.ps.Pause(d)
}

func (fd *failureDetector) probe(dest *discovery.Peer) error {
	msg := &pb.Ping{
		Nonce: genNonce(),
	}

	pong, err := fd.ps.Ping(dest.PingAddr, msg)
	if err != nil {
		dest.IncrementPing()
		if dest.NumPing() >= fd.maxFailedPings {
			return errDead
		}

		return err
	}

	pong.Signature = nil
	bytes, err := proto.Marshal(pong)
	if err != nil {
		return err
	}

	if sign := pong.GetSignature(); sign == nil {
		return errInvalidPongSignature
	} else {
		if valid := fd.cs.Verify(bytes, sign.GetR(), sign.GetS(), dest.PublicKey()); !valid {
			return errInvalidPongSignature
		}
	}

	dest.ResetPing()

	return nil
}

func (fd *failureDetector) start() {
	fd.ps.Start()
}

func (fd *failureDetector) stop() {
	fd.ps.Stop()
}

func genNonce() []byte {
	nonce := make([]byte, 32)
	rand.Read(nonce)
	return nonce
}
