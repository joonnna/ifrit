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
c, err := ifrit.NewClient()
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
After joining a Ifrit network you can send messages to anyone in it:

```go
members := client.Members()

randomMember := members[rand.Int() % len(members)]

ch := client.SendTo(randomMember, msg)

response := <-ch
```
The response will eventually be propagated through the returned channel.


To receive messages, you can register a message handler:
```go
client.RegisterMsgHandler(yourMessageHandler)

// This callback will be invoked on each received message.
func yourMessageHandler(data []byte) ([]byte, error) {
    // Do message logic
    return yourResponse, yourError
}
```
The response, or error if its non-nil, will be propagated back to the sender.


### Adding gossip
You can also gossip with neighboring peers in the Ifrit ring mesh. All incoming gossip is from neighbors, and all outgoing gossip is only sent to neighbors.
To add gossip, you simply attach your message to the client, it will then be sent to a neighboring peer at each gossip interval.
```go
client.SetGossipContent(yourGossipMsg)
```
To receive incoming gossip messages and responses you register two handlers:
```go
client.RegisterGossipHandler(yourGossipHandler)
client.RegisterResponseHandler(yourResponseHandler)

// This callback will be invoked on each received gossip message.
func yourGossipHandler(data []byte) ([]byte, error) {
    // Do your stuff
    return yourResponse, yourError
}


// This callback will be invoked on each received gossip response.
func yourResponseHandler(data []byte) {
    // Do your stuff
}
```
Note that gossip messages has seperate message and response handlers than that of normal messages.


### Config details
Ifrit clients relies on a config file which should either be placed in your current working directory or /var/tmp/ifrit_config.
Ifrit will generate all default values, but relies on two user inputs as explained earlier.
We will now present all configuration variables:
- use_ca (bool): if a ca should be contacted on startup.
- ca_addr (string): ip:port of the ca, has to be populated if use_ca is set to true.
- gossip_interval (uint32): How often (in seconds) the ifrit client should gossip with a neighboring peer (default: 10). Ifrit gossips with one neighbor per interval.
- monitor_interval (uint32): How often (in seconds) the ifrit client should monitor other peers (default: 10).
- ping_limit (uint32): How many failed pings before peers are considered dead (default: 3).
- max_concurrent_messages (uint32): The maximum concurrent outgoing messages through the messaging service at any time (default: 50).
- removal_timeout (uint32): How long (in seconds) the ifrit client waits after discovering an unresponsive peer before removing it from its live view (default: 60).
