package ifrit

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"

	_ "net/http/pprof"

	"github.com/joonnna/ifrit/cauth"
)

const (
	port = 5632
)

type Launcher struct {
	clientList      []*Client
	clientListMutex sync.RWMutex

	ca *cauth.Ca

	listener net.Listener

	entryAddr string
	numRings  uint32
}

func NewLauncher(numRings uint32) (*Launcher, error) {
	listener, err := netutil.ListenOnPort(netutil.GetLocalIP(), port)
	if err != nil {
		return nil, err
	}

	c, err := cauth.NewCa()
	if err != nil {
		listener.Close()
		return nil, err
	}

	l := Launcher{
		numRings:  numRings,
		entryAddr: c.GetAddr(),
		ca:        c,
	}

	return l, nil
}

func (l *Launcher) Start() {
	go l.ca.Start(l.numRings)

	http.HandleFunc("/addNode", l.addNodeHandler)

	http.Serve(l.listener, nil)
}

func (l *Launcher) ShutDown() {
	l.clientListMutex.RLock()
	defer l.clientListMutex.RUnlock()

	for _, c := range l.clientList {
		c.ShutDown()
	}

	l.listener.Close()
	l.ca.Shutdown()
}

func (l *Launcher) shutDownClients() {
	l.clientListMutex.RLock()
	defer l.clientListMutex.RUnlock()

	for _, c := range l.clientList {
		c.ShutDown()
	}
}

func (l *Launcher) startClient() {
	b.clientListMutex.Lock()
	defer b.clientListMutex.Unlock()

	c, err := NewClient(l.entryAddr, nil)
	if err != nil {
		fmt.Println(err)
		return
	}

	go c.Start()

	l.clientList = append(l.clientList, c)
}

func (l *Launcher) addNodeHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	defer r.Body.Close()

	l.startClient()
}
