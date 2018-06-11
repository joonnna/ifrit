# Ifrit
Go library implementing the Fireflies protocol, https://dl.acm.org/citation.cfm?id=2701418.
Ifrit provides a membership, message, signature, and gossip service.
Applications have a full membership view and can send messages directly to their destination.

[![GoDoc](https://godoc.org/github.com/joonnna/ifrit?status.svg)](https://godoc.org/github.com/joonnna/ifrit)

## Starting up a client and certificate authority
```go
package main

import (
	"github.com/joonnna/ifrit/cauth"
	"github.com/joonnna/ifrit"
)

func main() {
    ca, _ := cauth.NewCa()
    go ca.Start()

    c := client.NewClient()
    go c.Start()
}
```


## Example of sending messages
Mockup application periodically sending messages to a participant in the network.
```go
func (a *application) Start() {
    a.ifritClient.RegisterMsgHandler(a.handleMessages)

    for {
        a.doStuff()

        msg := a.newStuff()

        members := a.ifritClient.Members()

        ch := a.ifritClient.SendTo(members[randIdx], msg)

        resp := <-ch
        a.handleResp(resp)
    }
}

// This callback will be invoked on each received message.
func (a *application) handleMessages(data []byte) ([]byte, error) {
    msg := data.Unmarshal()

    a.storeMsg(msg)

    return a.generateResponse(msg), nil
}
```
