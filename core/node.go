package core

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/netutil"
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

	recordGossipRounds bool
	recordMutex        sync.RWMutex

	rounds     uint32
	roundMutex sync.RWMutex

	monitorTimeout  time.Duration
	nodeDeadTimeout float64

	msgHandler      processMsg
	msgHandlerMutex sync.RWMutex

	gossipHandler      processMsg
	gossipHandlerMutex sync.RWMutex

	responseHandler      func([]byte)
	responseHandlerMutex sync.RWMutex

	externalGossip      []byte
	externalGossipMutex sync.RWMutex

	dispatcher *workerpool.Dispatcher

	privKey    *ecdsa.PrivateKey
	localCert  *x509.Certificate
	caCert     *x509.Certificate
	entryAddrs []string

	stats *recorder

	trustedBootNode  bool
	vizId            string
	httpListener     net.Listener
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
	Addr() string
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
	Ca                 bool
	CaAddr             string
	EntryAddrs         []string
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
			if n.isGossipRecording() {
				n.incrementGossipRounds()
			}
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
	var i, mask uint32
	var extValue []byte
	var certs *certSet
	var http string
	var l net.Listener

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

	if conf.Visualizer {
		l, err = initHttp()
		if err != nil {
			log.Error(err.Error())
		}

		httpPort := strings.Split(l.Addr().String(), ":")[1]
		http = fmt.Sprintf("%s:%s", netutil.GetLocalIP(), httpPort)
	}

	if conf.Ca {
		addr := fmt.Sprintf("http://%s/certificateRequest", conf.CaAddr)
		certs, err = sendCertRequest(privKey, addr, s.Addr(), udpServer.Addr(), http)
		if err != nil {
			log.Error(err.Error())
			return nil, err
		}

	} else {
		// TODO only have numrings in notes and not certificate?
		certs, err = selfSignedCert(privKey, s.Addr(), udpServer.Addr(), http, uint32(32))
		if err != nil {
			log.Error(err.Error())
			return nil, err
		}
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
		entryAddrs:       conf.EntryAddrs,
		httpListener:     l,
		protocol:         correct{},
	}

	err = n.server.Init(config, n, ((n.numRings * 2) + 20))
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	n.client.Init(genClientConfig(certs, privKey))

	for i = 0; i < n.numRings; i++ {
		mask = setBit(mask, i)
	}

	localNote := &note{
		epoch:  1,
		mask:   mask,
		peerId: n.peerId,
	}

	err = localNote.sign(n.privKey)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	n.recentNote = localNote

	if n.caCert != nil {
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

func (n *Node) Sign(content []byte) ([]byte, []byte, error) {
	signature, err := signContent(content, n.privKey)
	if err != nil {
		return nil, nil, err
	}

	return signature.r, signature.s, nil
}

func (n *Node) Verify(r, s, content []byte, id string) bool {
	p := n.getViewPeer(id)
	if p == nil {
		return false
	}

	// TODO Cannot return error, change verifySignature
	valid, _ := validateSignature(r, s, content, p.publicKey)

	return valid
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

func (n *Node) LiveMembersHttp() []string {
	return n.getLivePeerHttpAddrs()
}

func (n *Node) Id() string {
	return n.key
}

func (n *Node) Addr() string {
	return n.server.Addr()
}

func (n *Node) HttpAddr() string {
	//return n.httpListener.Addr().String()
	return n.httpAddr
}

func (n *Node) Start() {
	log.Info("Started Node")

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
		go n.httpHandler()

		for _, r := range n.ringMap {
			for {
				err := n.add(r.ringNum)
				if err == nil {
					break
				}
			}
		}
	}

	// can continue even with error, will just send nil msg.
	msg, err := n.collectGossipContent()
	if err != nil {
		log.Error(err.Error())
	}

	// With no ca we need to contact existing hosts.
	// TODO retry if we fail to contact them?
	if n.caCert == nil {
		for _, addr := range n.entryAddrs {
			reply, err := n.client.Gossip(addr, msg)
			if err != nil {
				log.Error(err.Error(), "addr", addr)
				continue
			}

			n.mergeCertificates(reply.GetCertificates())
			n.mergeNotes(reply.GetNotes())
			n.mergeAccusations(reply.GetAccusations())
		}
	}

	<-n.exitChan
	n.httpListener.Close()
	n.server.ShutDown()
	n.pinger.shutdown()

	log.Info("Exiting node")
}
