# Ifrit
## How to install golang
https://golang.org/doc/install

## How to setup golang environment
https://golang.org/doc/code.html

## Client
```go
import github.com/joonnna/ifrit/ifrit
```

## Certificate authority
```go
import github.com/joonnna/ifrit/cauth
```

To download Ifrit simply issue the following command after including it in a src file "go get ./..."

## Starting up a client and certificate authority
```go
package main

import (
	"github.com/joonnna/ifrit/cauth"
	"github.com/joonnna/ifrit/ifrit"
)

func main() {
    var numRings uint32 = 5

    ca := cauth.NewCa()
    go ca.Start(numRings)

    c := client.NewClient(ca.GetAddr())
    go c.Start()

    doApplicationStuff(c)
}
```

## Application example of
In this example we aim to utilize the external gossip content functionality of ifrit.
Each time our application state is altered, we set the appropriate state in ifrit
through the SetGossipContent() function.
Our application state will then be gossiped with our neighbours which will receive it
through their registered message handlers.

The example is simplified for clarity reasons and is meant to represent an example usage
of the SetGossipContent() functionality.

```go
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

// As an example we store the client instance within the application
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

		case <-time.After(time.Second * 20):
			a.addRandomUser()
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

    //Updating state
	a.ifritClient.SetGossipContent(a.State())

    //No need to send a response, all application state will eventually converge
	return nil, nil
}

//We assume that the CA is deployed on another server
func main(caAddr string) {
	a, err := newApp(caAddr)
	if err != nil {
		panic(err)
	}

	a.Start()
}
```
