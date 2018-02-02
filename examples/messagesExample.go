package main

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/joonnna/ifrit/ifrit"
)

// Mockup application
type application struct {
	ifritClient *ifrit.Client
	caAddr      string

	exitChan chan bool
	data     *appData
}

// Mockup data structure
type appData struct {
	Users map[int]*user
}

// Mockup users
type user struct {
	FirstName string
	LastName  string
	Address   string
}

// We store the client instance within the application
// such that we can communicate with it as we see fit
func newApp(caAddr string) (*application, error) {
	c, err := ifrit.NewClient(caAddr)
	if err != nil {
		return nil, err
	}

	return &application{
		ifritClient: c,
		caAddr:      caAddr,
	}, nil
}

// Start the mockup application
func (a *application) Start() {
	a.ifritClient.RegisterMsgHandler(a.handleMessages)

	for {
		select {
		case <-a.exitChan:
			return

		case <-time.After(time.Sceond * 40):
			a.addRandomUser()
			ch, num := a.ifritClient.SenToAll(a.State())

			for i := 0; i < num; i++ {
				select {
				case resp <- ch:
					a.handleResponse(resp)
				case <-time.After(time.Minute * 2):
					break
				}
			}
		}
	}
}

func (a *application) State() []byte {
	var buf bytes.Buffer

	json.NewEncoder(buf).Encode(a.data)

	return buf.Bytes()
}

// This callback will be invoked on each received message.
func (a *application) handleMessages(data []byte) ([]byte, error) {
	received := &appData{}

	err := json.NewDecoder(bytes.NewReader(data)).Decode(received)
	if err != nil {
		return nil, err
	}

	for k, v := range received {
		if _, ok := a.data.Users[k]; !ok {
			a.data.Users[k] = v
		}
	}

	return a.generateResponse(), nil
}

//We assume that the CA is deployed on another server
func main(caAddr string) {
	a, err := newApp(caAddr)
	if err != nil {
		panic(err)
	}

	a.Start()
}
