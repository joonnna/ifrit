package core

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/joonnna/ifrit/log"
	"github.com/joonnna/ifrit/protobuf"
	"github.com/joonnna/ifrit/udp"
	"github.com/joonnna/workerpool"
)

var (
	errNoRingNum = errors.New("No ringnumber present in received certificate")
	errNoId      = errors.New("No id present in received certificate")
	errNoData    = errors.New("Gossip data has zero length")
)

/*
const (
	NormalProtocol          = 1
	SpamAccusationsProtocol = 2
	DosProtocol             = 3
)
*/

type processMsg func([]byte) ([]byte, error)

type Node struct {
	//Local peer representation
	*peer

	*view

	*pinger

	protocol
	protocolMutex sync.RWMutex

	client client
	server server

	wg       *sync.WaitGroup
	exitChan chan bool

	exitFlag  bool
	exitMutex sync.RWMutex

	gossipTimeout      time.Duration
	gossipTimeoutMutex sync.RWMutex

	monitorTimeout  time.Duration
	nodeDeadTimeout float64

	msgHandler      processMsg
	msgHandlerMutex sync.RWMutex

	responseHandler      func([]byte)
	responseHandlerMutex sync.RWMutex

	externalGossip      []byte
	externalGossipMutex sync.RWMutex

	dispatcher *workerpool.Dispatcher

	privKey   *ecdsa.PrivateKey
	localCert *x509.Certificate
	caCert    *x509.Certificate

	stats *recorder

	trustedBootNode  bool
	httpAddr         string
	viz              bool
	vizAddr          string
	vizUpdateTimeout uint32
}

type client interface {
	Init(config *tls.Config)
	Gossip(addr string, args *gossip.State) (*gossip.StateResponse, error)
	Dos(addr string, args *gossip.State) (*gossip.StateResponse, error)
	SendMsg(addr string, args *gossip.Msg) (*gossip.MsgResponse, error)
}

type server interface {
	Init(config *tls.Config, n interface{}, maxConcurrent uint32) error
	HostInfo() string
	Start() error
	ShutDown()
}

type protocol interface {
	Monitor(n *Node)
	Gossip(n *Node)
	Rebuttal(n *Node)
	Timeouts(n *Node)
}

type timeout struct {
	observer  *peer
	lastNote  *note
	timeStamp time.Time

	//For debugging
	addr string
}

type Config struct {
	EntryAddr          string
	GossipRate         uint32
	MonitorRate        uint32
	MaxFailPings       uint32
	ViewRemovalTimeout uint32
	Visualizer         bool
	VisAddr            string
	VisUpdateTimeout   uint32
	MaxConc            uint32
}

func (n *Node) gossipLoop() {
	for {
		select {
		case <-n.exitChan:
			log.Info("Exiting gossiping")
			n.wg.Done()
			return
		case <-time.After(n.getGossipTimeout()):
			n.getProtocol().Gossip(n)
		}
	}
}

func (n *Node) monitor() {
	for {
		select {
		case <-n.exitChan:
			log.Info("Stopping monitoring")
			n.wg.Done()
			return
		case <-time.After(n.monitorTimeout):
			n.getProtocol().Monitor(n)
		}
	}
}

func (n *Node) checkTimeouts() {
	for {
		select {
		case <-n.exitChan:
			log.Info("Stopping view update")
			n.wg.Done()
			return
		case <-time.After(n.viewUpdateTimeout):
			n.getProtocol().Timeouts(n)
		}
	}
}

func NewNode(conf *Config, c client, s server) (*Node, error) {
	var i uint32
	var extValue []byte

	udpServer, err := udp.NewServer()
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	privKey, err := genKeys()
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	addr := fmt.Sprintf("http://%s/certificateRequest", conf.EntryAddr)

	certs, err := sendCertRequest(addr, privKey, s.HostInfo(), udpServer.Addr())
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	extensions := certs.ownCert.Extensions

	if len(certs.ownCert.SubjectKeyId) < 1 {
		log.Error(errNoId.Error())
		return nil, errNoId
	}

	for _, e := range extensions {
		if e.Id.Equal(asn1.ObjectIdentifier{2, 5, 13, 37}) {
			extValue = e.Value
		}
	}
	if extValue == nil {
		log.Error(errNoRingNum.Error())
		return nil, errNoRingNum
	}

	numRings := binary.LittleEndian.Uint32(extValue[0:])

	config := genServerConfig(certs, privKey)

	p, err := newPeer(nil, certs.ownCert, numRings)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	v, err := newView(numRings, p.peerId, p.addr)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	n := &Node{
		exitChan:         make(chan bool, 1),
		wg:               &sync.WaitGroup{},
		gossipTimeout:    time.Second * time.Duration(conf.GossipRate),
		monitorTimeout:   time.Second * time.Duration(conf.MonitorRate),
		nodeDeadTimeout:  float64(conf.ViewRemovalTimeout),
		view:             v,
		pinger:           newPinger(udpServer, conf.MaxFailPings, privKey),
		privKey:          privKey,
		client:           c,
		server:           s,
		peer:             p,
		stats:            &recorder{recordDuration: 60},
		localCert:        certs.ownCert,
		caCert:           certs.caCert,
		trustedBootNode:  certs.trusted,
		viz:              conf.Visualizer,
		vizUpdateTimeout: conf.VisUpdateTimeout,
		vizAddr:          conf.VisAddr,
		dispatcher:       workerpool.NewDispatcher(conf.MaxConc),
	}

	err = n.server.Init(config, n, ((n.numRings * 2) + 20))
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	n.client.Init(genClientConfig(certs, privKey))

	localNote := &note{
		epoch:  1,
		mask:   make([]byte, numRings),
		peerId: n.peerId,
	}
	for i = 0; i < n.numRings; i++ {
		localNote.mask[i] = 1
	}

	err = localNote.sign(n.privKey)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	n.recentNote = localNote

	for _, c := range certs.knownCerts {
		if n.peerId.equal(&peerId{id: c.SubjectKeyId}) {
			continue
		}
		n.evalCertificate(c)
	}

	view := n.getView()

	for _, p := range view {
		n.addLivePeer(p)
	}

	return n, nil
}

func (n *Node) SendMessage(dest string, ch chan []byte, data []byte) {
	msg := &gossip.Msg{
		Content: data,
	}

	n.dispatcher.Submit(func() {
		n.sendMsg(dest, ch, msg)
	})
}

func (n *Node) SendMessages(dest []string, ch chan []byte, data []byte) {
	msg := &gossip.Msg{
		Content: data,
	}

	for _, addr := range dest {
		a := addr
		n.dispatcher.Submit(func() {
			n.sendMsg(a, ch, msg)
		})
	}
}

func (n *Node) sendMsg(dest string, ch chan []byte, msg *gossip.Msg) {
	reply, err := n.client.SendMsg(dest, msg)
	if err != nil {
		log.Error(err.Error())
		ch <- nil
	}
	ch <- reply.GetContent()
}

func (n *Node) ShutDownNode() {
	if n.viz {
		for _, r := range n.ringMap {
			n.remove(r.ringNum)
		}
	}

	close(n.exitChan)
	n.dispatcher.Stop()
	n.wg.Wait()
}

func (n *Node) LiveMembers() []string {
	return n.getLivePeerAddrs()
}

func (n *Node) Id() string {
	return n.key
}

func (n *Node) Start() {
	log.Info("Started Node")

	done := make(chan bool)

	n.setProtocol(correct{})

	go n.server.Start()
	go n.pinger.serve()

	routines := 3
	if n.viz {
		routines = 4
	}

	n.wg.Add(routines)
	go n.gossipLoop()
	go n.monitor()
	go n.checkTimeouts()

	n.dispatcher.Start()

	if n.viz {
		go n.updateState()
		go n.httpHandler(done)

		<-done

		for _, r := range n.ringMap {
			for {
				err := n.add(r.ringNum)
				if err == nil {
					break
				}
			}
		}
	}

	<-n.exitChan
	n.server.ShutDown()
	n.pinger.shutdown()

	log.Info("Exiting node")
}
