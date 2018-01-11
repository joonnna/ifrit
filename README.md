## How to install golang 
https://golang.org/doc/install

## How to setup golang environment 
https://golang.org/doc/code.html

## Client
import github.com/joonnna/ifrit/client

## Certificate authority
import github.com/joonnna/ifrit/cauth

To download Ifrit simply issue the following command after including it in a src file "go get ./..."

## An example usage

```go
package main

import (
	"github.com/joonnna/ifrit/cauth"
	"github.com/joonnna/ifrit/client"	
)

func main() {
	numClients := 10
	numRings := 5

	ca := cauth.NewCa()
	go ca.Start(numRings)

	for i := 0; i < numClients; i++ {
		c := client.NewClient(ca.Addr())
		go c.Start()
	}
}
```

