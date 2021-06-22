package core

import (
	"crypto/ecdsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"path"
	"sync"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit/core/discovery"
	pb "github.com/joonnna/ifrit/protobuf"
	"github.com/joonnna/workerpool"
	"github.com/spf13/viper"
)

var (
	errNoId         = errors.New("No id present in received certificate")
	errNoData       = errors.New("Gossip data has zero length")
	errNoCaAddr     = errors.New("No ca addr set in config with use_ca enabled")
	errNoEntryAddrs = errors.New("No entry_addrs set in config with use_ca disabled")
	errInvlKeyPath  = errors.New("Storage path-argument is invalid")
)

type processMsg func([]byte) ([]byte, error)
type streamMsg func(chan []byte, chan []byte)

type Node struct {
	view *discovery.View
	self *discovery.Peer

	p             protocol
	protocolMutex sync.RWMutex

	wg       *sync.WaitGroup
	exitChan chan bool

	exitFlag  bool
	exitMutex sync.RWMutex

	viewUpdateTimeout time.Duration

	gossipTimeout      time.Duration
	gossipTimeoutMutex sync.RWMutex

	pingsPerInterval int
	monitorTimeout   time.Duration
	nodeDeadTimeout  float64

	msgHandler      processMsg
	msgHandlerMutex sync.RWMutex

	gossipHandler      processMsg
	gossipHandlerMutex sync.RWMutex

	responseHandler      func([]byte)
	responseHandlerMutex sync.RWMutex

	externalGossip      []byte
	externalGossipMutex sync.RWMutex

	streamHandler      streamMsg
	streamHandlerMutex sync.RWMutex

	dispatcher *workerpool.Dispatcher

	entryAddrs []string

	fd *failureDetector

	comm commService
	cs   cryptoService
	cm   certManager

	useViz bool
	viz    *viz
}

type commService interface {
	Register(pb.GossipServer)
	CloseConn(string)
	Addr() string
	Start()
	Stop()

	Gossip(string, *pb.State) (*pb.StateResponse, error)
	Send(string, *pb.Msg) (*pb.MsgResponse, error)
	StreamMessenger(string, chan []byte, chan []byte) error
}

type certManager interface {
	Certificate() *x509.Certificate
	CaCertificate() *x509.Certificate
	Priv() *ecdsa.PrivateKey // Added for Saving private-key.
	ContactList() []*x509.Certificate
	NumRings() uint32
	Trusted() bool
}

type cryptoService interface {
	Verify([]byte, []byte, []byte, *ecdsa.PublicKey) bool
	Sign([]byte) ([]byte, []byte, error)
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
			n.protocol().Gossip(n)
		}
	}
}

func (n *Node) monitorLoop() {
	defer n.wg.Done()

	for {
		select {
		case <-n.exitChan:
			log.Info("Stopping monitoring")
			return
		case <-time.After(n.monitorTimeout):
			n.protocol().Monitor(n)
		}
	}
}

func NewNode(comm commService, ps pingService, cm certManager, cs cryptoService) (*Node, error) {
	var perInterval int

	v, err := discovery.NewView(cm.NumRings(), cm.Certificate(), comm, cs)
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
		exitChan:       make(chan bool, 1),
		wg:             &sync.WaitGroup{},
		gossipTimeout:  time.Second * time.Duration(viper.GetInt32("gossip_interval")),
		monitorTimeout: time.Second * time.Duration(viper.GetInt32("monitor_interval")),
		dispatcher: workerpool.NewDispatcher(uint32(viper.
			GetInt32("max_concurrent_messages"))),
		entryAddrs:       viper.GetStringSlice("entry_addrs"),
		p:                correct{},
		pingsPerInterval: perInterval,

		fd:   newFd(ps, cs, uint32(viper.GetInt32("ping_limit"))),
		cm:   cm,
		cs:   cs,
		comm: comm,
		self: v.Self(),
		view: v,

		// Visualizer specific
		useViz: viper.GetBool("use_viz"),
	}

	if n.useViz {
		interval := time.Second * time.Duration(viper.GetInt32("viz_update_interval"))
		viz, err := newViz(n, viper.GetString("viz_addr"), interval, cm.Trusted())
		if err != nil {
			return nil, err
		}
		n.viz = viz
	}

	n.comm.Register(n)

	if n.cm.CaCertificate() != nil {
		for _, c := range n.cm.ContactList() {
			if n.self.Id == string(c.SubjectKeyId) {
				continue
			}
			err := n.evalCertificate(c)
			if err != nil {
				log.Error(err.Error())
			}
		}

		// We add all our initial contacts to the live view without notes
		// to ensure that we have someone to gossip with at startup.
		for _, p := range n.view.Full() {
			n.view.AddLive(p)
		}
	}

	return n, nil
}

func (n *Node) SendMessage(dest string, ch chan []byte, data []byte) {
	msg := &pb.Msg{
		Content: data,
	}

	n.dispatcher.Submit(func() {
		n.sendMsg(dest, ch, msg)
	})
}

func (n *Node) Sign(content []byte) ([]byte, []byte, error) {
	r, s, err := n.cs.Sign(content)
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

	return n.cs.Verify(content, r, s, p.PublicKey())
}

func (n *Node) IdToAddr(id []byte) (string, error) {
	p := n.view.Peer(string(id))
	if p == nil {
		return "", errors.New("Could not find peer with specified id")
	}

	return p.Addr, nil
}

func (n *Node) SendMessages(dest []string, ch chan []byte, data []byte) {
	msg := &pb.Msg{
		Content: data,
	}

	for _, addr := range dest {
		a := addr
		n.dispatcher.Submit(func() {
			n.sendMsg(a, ch, msg)
		})
	}
}

func (n *Node) OpenStream(dest string, input, reply chan []byte) {
	n.dispatcher.Submit(func() {
		n.openStream(dest, input, reply)
	})
}

func (n *Node) SendStream(ch chan<- []byte, data []byte) {
	ch <- data
}

func (n *Node) openStream(dest string, input, reply chan []byte) {
	if err := n.comm.StreamMessenger(dest, input, reply); err != nil {
		log.Error(err.Error())
	}
}

func (n *Node) sendMsg(dest string, ch chan []byte, msg *pb.Msg) {
	reply, err := n.comm.Send(dest, msg)
	if err != nil {
		log.Error(err.Error())
		ch <- nil
	}
	ch <- reply.GetContent()
}

func (n *Node) isStopping() bool {
	n.exitMutex.Lock()
	defer n.exitMutex.Unlock()

	if n.exitFlag {
		return true
	}

	n.exitFlag = true
	close(n.exitChan)

	return false
}

func (n *Node) Stop() {
	if n.isStopping() {
		return
	}

	if n.useViz {
		n.viz.stop()
	}

	n.view.Stop()
	n.fd.stop()

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
	return n.comm.Addr()
}

func (n *Node) Start() {
	log.Info("Started Node")

	go n.fd.start()
	go n.comm.Start()
	go n.view.Start()

	n.wg.Add(2)
	go n.gossipLoop()
	go n.monitorLoop()

	n.dispatcher.Start()

	if n.useViz {
		n.viz.start()
	}

	msg := n.collectGossipContent()

	// With no ca we need to contact existing hosts.
	// TODO retry if we fail to contact them?
	if n.cm.CaCertificate() == nil {
		for _, addr := range n.entryAddrs {
			reply, err := n.comm.Gossip(addr, msg)
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
	log.Info("Exiting node")
	n.Stop()
}

/* Save private key for node crypto-unit to new file in argument directory-path.
 * - marius
 */
func (n *Node) SavePrivateKey(p string) error {
	if p == "" {
		return errInvlKeyPath
	}

	p = path.Join(p, "key.pem")

	f, err := os.Create(p)
	if err != nil {
		return err
	}

	keyBytes, err := x509.MarshalECPrivateKey(n.cm.Priv())
	if err != nil {
		return err
	}

	block := &pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	}

	err = pem.Encode(f, block)
	if err != nil {
		return err
	}

	return f.Close()
}

/* Save certificates for network-neighbours in new files inside argument path.
 * - marius
 */
func (n *Node) SaveCertificates(p string) error {
	if p == "" {
		return errInvlKeyPath
	}

	for _, cert := range n.cm.ContactList() {

		certPath := path.Join(p, fmt.Sprintf("g-%s.pem", cert.SerialNumber))

		f, err := os.Create(certPath)
		if err != nil {
			log.Error(err.Error())
			continue
		}

		certBytes := cert.Raw

		block := &pem.Block{
			Type:  "CERTIFICATE",
			Bytes: certBytes,
		}

		if err = pem.Encode(f, block); err != nil {
			log.Error(err.Error())
		}

		if err = f.Close(); err != nil {
			log.Error(err.Error())
		}
	}

	return nil
}
