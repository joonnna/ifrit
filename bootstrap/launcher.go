package bootstrap

import (
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"sync"

	_ "net/http/pprof"

	"github.com/joonnna/ifrit/cauth"
	"github.com/joonnna/ifrit/worm"

	log "github.com/inconshreveable/log15"
)

const (
	port = 5632
)

type Launcher struct {
	applicationList      []application
	applicationListMutex sync.RWMutex

	ca *cauth.Ca

	listener net.Listener

	// TODO naming things...
	worm *worm.Worm

	ch chan interface{}
}

type application interface {
	Start()
	ShutDown()
	Addr() string
	HttpAddr() string
}

func NewLauncher(ch chan interface{}, w *worm.Worm) (*Launcher, error) {
	listener, err := net.Listen("tcp", fmt.Sprintf("localhost:%d", port))
	if err != nil {
		return nil, err
	}

	c, err := cauth.NewCa()
	if err != nil {
		listener.Close()
		return nil, err
	}

	l := &Launcher{
		ca:       c,
		ch:       ch,
		listener: listener,
		worm:     w,
	}

	return l, nil
}

func (l *Launcher) Start() {
	go l.ca.Start()
	if l.worm != nil {
		log.Info("Starting worm")
		l.worm.Start()
	}
	http.HandleFunc("/addApplication", l.addApplicationHandler)

	http.Serve(l.listener, nil)
}

func (l *Launcher) ShutDown() {
	l.shutDownApplications()
	if l.worm != nil {
		l.worm.Stop()
	}
	l.listener.Close()
	l.ca.Shutdown()
}

func (l *Launcher) Addr() string {
	return l.listener.Addr().String()
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

	//Hacky...
	l.ch <- 0

	instance := <-l.ch

	client, ok := instance.(application)
	if !ok {
		fmt.Println("Invalid interface")
		return
	}

	go client.Start()

	if len(l.applicationList) <= 0 && l.worm != nil {
		l.worm.AddHost(client.HttpAddr())
	}

	l.applicationList = append(l.applicationList, client)
}

func (l *Launcher) addApplicationHandler(w http.ResponseWriter, r *http.Request) {
	io.Copy(ioutil.Discard, r.Body)
	defer r.Body.Close()

	l.startApplication()
}
