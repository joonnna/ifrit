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

## An example usage

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

