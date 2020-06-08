package main

import (
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joonnna/ifrit"
)

/** Example of bi-directional streaming using Ifrit library*/

type App struct {
	master  *ifrit.Client
	network []*ifrit.Client
}

func main() {	
	numClient := 5
	app, err := NewApp(numClient)
	if err != nil {
		panic(err)
	}

	app.Stream()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	app.master.Stop()
}

func NewApp(size int) (*App, error) {
	network := make([]*ifrit.Client, 0)

	// Add all clients to network
	for i := 0; i < size; i++ {
		fmt.Println("Adding client...")
		c, err := ifrit.NewClient()
		if err != nil {
			return nil, err
		}

		network = append(network, c)
		go c.Start()
		c.RegisterStreamHandler(streamHandler)
	}

	master, err := ifrit.NewClient()
	if err != nil {
		return nil, err
	}

	go master.Start()
	return &App{
		master:  master,
		network: network,
	}, nil
}

func (app *App) Stream() {
	members := app.master.Members()
	randomMember := members[rand.Int()%len(members)]
	var wg sync.WaitGroup

	inputStream, reply := app.master.OpenStream(randomMember, 10, 10)

	// Test the input stream
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Second)
		for i := 0; i < 10; i++ {
			msg := fmt.Sprintf("Client message %d", i)
			inputStream <- []byte(msg)
		}

		close(inputStream)
	}()

	// Test the reply stream
	wg.Add(1)
	go func() {
		time.Sleep(1 * time.Second)
		defer wg.Done()
		for i := range reply {
			fmt.Printf("Client got reply from server: %s\n", i)
		}
	}()

	wg.Wait()
}

func streamHandler(input, reply chan []byte) {
	var wg sync.WaitGroup

	// Read messages from client
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range input {
			fmt.Printf("Server got message: %s\n", i)
		}
	}()

	// Send replies to the client
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 9; i++ {
			msg := fmt.Sprintf("Server reply %d", i)
			reply <- []byte(msg)
		}
		close(reply)
	}()

	wg.Wait()
}

