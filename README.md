# Ifrit
Go library implementing the Fireflies protocol, https://dl.acm.org/citation.cfm?id=2701418.
Ifrit provides a membership, message, signature, and gossip service.
Applications have a full membership view and can send messages directly to their destination.

[![GoDoc](https://godoc.org/github.com/joonnna/ifrit?status.svg)](https://godoc.org/github.com/joonnna/ifrit)

### Starting a Ifrit network
To start a Ifrit network, you need to deploy the Ifrit certificate authority, responsible for signing certificates and acting as an entry point into the network.

The certificate authority serves a single HTTP endpoint for certificate signing requests, piggybacking certificates of existing participants on responses.
To start a network, simply deploy it.

Firstly, import the certificate authority package:
```go
"github.com/joonnna/ifrit/cauth"
```
Then create and start a ca instance:
```go
ca, err := cauth.NewCa()
if err != nil {
    panic(err)
}

go ca.Start()

```

Furthermore, you can customize parameters by changing the generated config file, located at /var/tmp/ca_config.
It contains three parameters: boot_nodes, num_rings, and port.
- boot_nodes: how many participant certificates the certificate authority stores locally (default: 10).
- num_rings: how many Ifrit rings will be used by participants, embedded in all certificates (default: 10).
- port: which port to bind to (default: 8300).




### Starting a client
Firstly you'll have to populate the client config with the address (ip:port) of the certificate authority.
Create the file "ifrit_config.yml", and place it under the /var/tmp folder.
Fill in the following variables:
- use_ca: true
- ca_addr: "insert ca address here"


Now you'll want to import the library:
```go
"github.com/joonnna/ifrit"
```
Now you can create and start a client
```go
c, err := client.NewClient()
if err != nil {
    panic(err)
}

go c.Start()

```
After participating in the network for some time you will learn of all other participants in the network, you can retreive their addresses as follows:
```go
allNetworkMembers := c.Members()
```


### Sending a message
You can send messages to anyone in the network:

```go
members := client.Members()

randomMember := members[rand.Int() % len(members)]

ch := client.SendTo(randomMember, msg)

response := <-ch
```
The response will eventually be propagated through the returned channel.


To receive messages, you can register a message handler:
```go
client.RegisterMsgHandler(youMessageHandler)

// This callback will be invoked on each received message.
func yourMessageHandler(data []byte) ([]byte, error) {
    // Do message logic
    return response, error
}
```
The response, or error if its non-nil, will be propagated back to the sender.


### Adding gossip
```go
client.RegisterGossipHandler(handleGossip)
client.RegisterResponseHandler(handleResponse)

gossipMsg := newMsg()

client.SetGossipContent(content)


// This callback will be invoked on each received gossip message.
func handleGossip(data []byte) ([]byte, error) {
    msg := data.Unmarshal()

    storeMsg(msg)

    return generateResponse(msg), nil
}


// This callback will be invoked on each received gossip response.
func handleResponse(data []byte) {
    resp := data.Unmarshal()

    storeResp(resp)
}

```
