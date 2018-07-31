package core

import (
	"crypto/ecdsa"
	"errors"

	"github.com/golang/protobuf/proto"
	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/core/discovery"
	"github.com/joonnna/ifrit/protobuf"
)

var (
	errInvalidPongSignature = errors.New("Invalid signature on pong message")
	errDead                 = errors.New("Peer is dead")
)

type pinger struct {
	exitChan chan bool
	transport
	privKey        *ecdsa.PrivateKey
	maxFailedPings uint32
}

type transport interface {
	Send(addr string, data []byte) ([]byte, error)
	Serve(func(data []byte) ([]byte, error)) error
	Pause(d time.Duration)
	Stop()
}

func newPinger(t transport, maxPing uint32, priv *ecdsa.PrivateKey) *pinger {
	return &pinger{
		transport:      t,
		privKey:        priv,
		maxFailedPings: maxPing,
	}
}

func (p *pinger) stopServing(d time.Duration) {
	p.Pause(d)
}

func (p *pinger) ping(dest *discovery.Peer) error {
	msg := &gossip.Ping{
		Nonce: genNonce(),
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return err
	}

	respBytes, err := p.Send(dest.PingAddr, data)
	if err != nil {
		dest.IncrementPing()
		if dest.NumPing() >= p.maxFailedPings {
			return errDead
		}

		return err
	}

	resp := &gossip.Pong{}

	err = proto.Unmarshal(respBytes, resp)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	sign := resp.GetSignature()

	if valid := dest.ValidateSignature(sign.GetR(), sign.GetS(), resp.GetNonce()); !valid {
		log.Error(errInvalidPongSignature.Error())
		return errInvalidPongSignature
	}

	dest.ResetPing()

	return nil
}

func (p pinger) signPong(data []byte) ([]byte, error) {
	r, s, err := signContent(data, p.privKey)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	resp := &gossip.Pong{
		Nonce: data,
		Signature: &gossip.Signature{
			R: r,
			S: s,
		},
	}

	bytes, err := proto.Marshal(resp)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	return bytes, nil
}

func (p pinger) serve() {
	p.Serve(p.signPong, p.exitChan)
}

func (p *pinger) shutdown() {
	close(p.exitChan)
	p.Shutdown()
}
