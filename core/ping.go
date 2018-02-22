package core

import (
	"crypto/ecdsa"
	"errors"

	"github.com/golang/protobuf/proto"
	log "github.com/inconshreveable/log15"
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
	Serve(func(data []byte) ([]byte, error), chan bool) error
	Shutdown()
}

func newPinger(t transport, maxPing uint32, priv *ecdsa.PrivateKey) *pinger {
	return &pinger{
		transport:      t,
		privKey:        priv,
		exitChan:       make(chan bool),
		maxFailedPings: maxPing,
	}
}

func (p *pinger) ping(dest *peer) error {
	if dest.getNPing() >= p.maxFailedPings {
		return errDead
	}

	msg := &gossip.Ping{
		Nonce: genNonce(),
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	dest.incrementPing()

	respBytes, err := p.Send(dest.pingAddr, data)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	resp := &gossip.Pong{}

	err = proto.Unmarshal(respBytes, resp)
	if err != nil {
		log.Error(err.Error())
		return err
	}
	sign := resp.GetSignature()

	valid, err := validateSignature(sign.GetR(), sign.GetS(), resp.GetNonce(), dest.publicKey)
	if err != nil {
		log.Error(err.Error())
		return err
	}

	if !valid {
		return errInvalidPongSignature
	}

	dest.setAvgLoss()
	dest.resetPing()

	return nil
}

func (p pinger) signPong(data []byte) ([]byte, error) {
	sign, err := signContent(data, p.privKey)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	resp := &gossip.Pong{
		Nonce: data,
		Signature: &gossip.Signature{
			R: sign.r,
			S: sign.s,
		},
	}

	//p.log.Debug.Println("sign len : ", (len(sign.r) + len(sign.s)))

	bytes, err := proto.Marshal(resp)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	//p.log.Debug.Println("sign len : ", (len(bytes)))

	return bytes, nil
}

func (p pinger) serve() {
	p.Serve(p.signPong, p.exitChan)
}

func (p *pinger) shutdown() {
	close(p.exitChan)
	p.Shutdown()
}
