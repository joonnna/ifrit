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
	"net/http"
	"strings"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/core/discovery"
	"github.com/joonnna/ifrit/netutil"
	"github.com/joonnna/ifrit/protobuf"
	"github.com/joonnna/ifrit/udp"
	"github.com/joonnna/workerpool"
	"github.com/spf13/viper"
)

var (
	errNoRingNum    = errors.New("No ringnumber present in received certificate")
	errNoId         = errors.New("No id present in received certificate")
	errNoData       = errors.New("Gossip data has zero length")
	errNoCaAddr     = errors.New("No ca addr set in config with use_ca enabled")
	errNoEntryAddrs = errors.New("No entry_addrs set in config with use_ca disabled")
)

type processMsg func([]byte) ([]byte, error)

type Node struct {
	view *discovery.View
	self *discovery.Peer

	failureDetector *pinger

	protocol      protocol
	protocolMutex sync.RWMutex

	client client
	server server

	wg       *sync.WaitGroup
	exitChan chan bool

	exitFlag  bool
	exitMutex sync.RWMutex

	viewUpdateTimeout time.Duration
	pingsPerInterval  int

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
	httpServer       *http.Server
	httpAddr         string
	viz              bool
	vizAddr          string
	vizAppAddr       string
	vizUpdateTimeout time.Duration
}

type client interface {
	Init(config *tls.Config)
	Gossip(addr string, args *gossip.State) (*gossip.StateResponse, error)
	SendMsg(addr string, args *gossip.Msg) (*gossip.MsgResponse, error)
	CloseConn(addr string)
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
}

func (n *Node) gossipLoop() {
	defer n.wg.Done()

	for {
		select {
		case <-n.exitChan:
			log.Info("Exiting gossiping")
			return
		case <-time.After(n.getGossipTimeout()):
			n.getProtocol().Gossip(n)
		}
	}
}

func (n *Node) monitor() {
	defer n.wg.Done()

	for {
		select {
		case <-n.exitChan:
			log.Info("Stopping monitoring")
			return
		case <-time.After(n.monitorTimeout):
			n.getProtocol().Monitor(n)
		}
	}
}

func (n *Node) checkTimeouts() {
	defer n.wg.Done()

	for {
		select {
		case <-n.exitChan:
			log.Info("Stopping view update")
			return
		case <-time.After(n.viewUpdateTimeout):
			n.view.CheckTimeouts()
		}
	}
}

func NewNode(c client, s server) (*Node, error) {
	var extValue []byte
	var certs *certSet
	var http string
	var l net.Listener
	var perInterval int

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

	if useViz := viper.GetBool("use_viz"); useViz {
		l, err = initHttp()
		if err != nil {
			log.Error(err.Error())
		} else {
			httpPort := strings.Split(l.Addr().String(), ":")[1]
			http = fmt.Sprintf("%s:%s", netutil.GetLocalIP(), httpPort)
		}
	}

	if useCa := viper.GetBool("use_ca"); useCa {
		if exists := viper.IsSet("ca_addr"); !exists {
			return nil, errNoCaAddr
		}

		addr := fmt.Sprintf("http://%s/certificateRequest", viper.GetString("ca_addr"))
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

	for _, e := range certs.ownCert.Extensions {
		if e.Id.Equal(asn1.ObjectIdentifier{2, 5, 13, 37}) {
			extValue = e.Value
		}
	}

	if extValue == nil {
		log.Error(errNoRingNum.Error())
		return nil, errNoRingNum
	}

	numRings := binary.LittleEndian.Uint32(extValue[0:])

	v, err := discovery.NewView(numRings, certs.ownCert, privKey, c.CloseConn)
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	num := viper.GetInt("pings_per_interval")
	if num == 0 {
		perInterval = 1
	} else if rings := int(v.NumRings()); num > rings {
		perInterval = rings
	} else {
		perInterval = num
	}

	n := &Node{
		exitChan:          make(chan bool, 1),
		wg:                &sync.WaitGroup{},
		gossipTimeout:     time.Second * time.Duration(viper.GetInt32("gossip_interval")),
		monitorTimeout:    time.Second * time.Duration(viper.GetInt32("monitor_interval")),
		viewUpdateTimeout: time.Second * time.Duration(viper.GetInt32("view_update_interval")),
		view:              v,
		failureDetector:   newPinger(udpServer, uint32(viper.GetInt32("ping_limit")), privKey),
		privKey:           privKey,
		client:            c,
		server:            s,
		stats:             &recorder{recordDuration: 60},
		localCert:         certs.ownCert,
		caCert:            certs.caCert,
		dispatcher:        workerpool.NewDispatcher(uint32(viper.GetInt32("max_concurrent_messages"))),
		entryAddrs:        viper.GetStringSlice("entry_addrs"),
		httpListener:      l,
		protocol:          correct{},
		self:              v.Self(),
		pingsPerInterval:  perInterval,

		// Visualizer specific
		viz:              viper.GetBool("use_viz"),
		vizAddr:          viper.GetString("viz_addr"),
		vizId:            fmt.Sprintf("http://%s", http),
		trustedBootNode:  certs.trusted,
		vizUpdateTimeout: time.Second * time.Duration(viper.GetInt32("viz_update_interval")),
	}

	serverConfig := genServerConfig(certs, privKey)

	err = n.server.Init(serverConfig, n, ((numRings * 2) + 20))
	if err != nil {
		log.Error(err.Error())
		return nil, err
	}

	n.client.Init(genClientConfig(certs, privKey))

	if n.caCert != nil {
		for _, c := range certs.knownCerts {
			if n.self.Id == string(c.SubjectKeyId) {
				continue
			}
			n.evalCertificate(c)
		}

		for _, p := range n.view.Full() {
			n.view.AddLive(p)
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
	r, s, err := signContent(content, n.privKey)
	if err != nil {
		return nil, nil, err
	}

	return r, s, nil
}

func (n *Node) Verify(r, s, content []byte, id string) bool {
	p := n.view.Peer(id)
	if p == nil {
		return false
	}

	return p.ValidateSignature(r, s, content)
}

func (n *Node) IdToAddr(id []byte) (string, error) {
	p := n.view.Peer(string(id))
	if p == nil {
		return "", errors.New("Could not find peer with specified id")
	}

	return p.Addr, nil
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
	close(n.exitChan)

	n.dispatcher.Stop()
	n.wg.Wait()
}

func (n *Node) LiveMembers() []string {
	live := n.view.Live()

	ret := make([]string, 0, len(live))

	for _, p := range n.view.Live() {
		ret = append(ret, p.Addr)
	}

	return ret
}

func (n *Node) HttpAddr() string {
	return n.self.HttpAddr
}

func (n *Node) Id() string {
	return n.self.Id
}

func (n *Node) Addr() string {
	return n.server.Addr()
}

func (n *Node) Start() {
	log.Info("Started Node")

	go n.server.Start()
	go n.failureDetector.serve()

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
		n.addToViz()
		go n.updateState()
		go n.httpHandler()
	}

	msg := n.collectGossipContent()

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
	n.httpServer.Close()
	n.server.ShutDown()
	n.failureDetector.shutdown()

	log.Info("Exiting node")

	if n.viz {
		n.remove()
	}
}
