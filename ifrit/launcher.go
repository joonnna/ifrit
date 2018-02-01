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
	applicationList      []application
	applicationListMutex sync.RWMutex

	ca *cauth.Ca

	listener net.Listener

	EntryAddr string
	numRings  uint32

	ch chan interface{}
}

type application interface {
	Start()
	ShutDown()
}

func NewLauncher(numRings uint32, ch chan interface{}) (*Launcher, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return nil, err
	}

	c, err := cauth.NewCa()
	if err != nil {
		listener.Close()
		return nil, err
	}

	l := &Launcher{
		numRings:  numRings,
		EntryAddr: c.GetAddr(),
		ca:        c,
		ch:        ch,
		listener:  listener,
	}

	return l, nil
}

func (l *Launcher) Start() {
	go l.ca.Start(l.numRings)

	http.HandleFunc("/addApplication", l.addApplicationHandler)

	http.Serve(l.listener, nil)
}

func (l *Launcher) ShutDown() {
	l.shutDownApplications()
	l.listener.Close()
	l.ca.Shutdown()
}

func (l *Launcher) shutDownApplications() {
	l.applicationListMutex.RLock()
	defer l.applicationListMutex.RUnlock()

	for _, n := range l.applicationList {
		n.ShutDown()
	}
}

func (l *Launcher) startApplication() {
	l.applicationListMutex.Lock()
	defer l.applicationListMutex.Unlock()
	/*
		c, err := NewApplication(l.entryAddr, nil)
		if err != nil {
			fmt.Println(err)
			return
		}

		go c.Start()
	*/

	l.ch <- 0

	instance := <-l.ch

	client, ok := instance.(application)
	if !ok {
		fmt.Println("Invalid interface")
		return
	}

	go client.Start()

	l.applicationList = append(l.applicationList, client)
}

func (l *Launcher) addApplicationHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	defer r.Body.Close()

	l.startApplication()
}
