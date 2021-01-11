package main

import (
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	log "github.com/inconshreveable/log15"
	"github.com/joonnna/ifrit"
)

/** Example of bi-directional streaming using Ifrit library*/

type App struct {
	master *ifrit.Client
	client *ifrit.Client
}

func main() {
	r := log.Root()
	h := log.CallerFileHandler(log.Must.FileHandler("logfile", log.LogfmtFormat()))
	r.SetHandler(h)

	app, err := NewApp()
	if err != nil {
		panic(err)
	}

	go app.Stream()

	channel := make(chan os.Signal, 2)
	signal.Notify(channel, os.Interrupt, syscall.SIGTERM)
	<-channel

	app.master.Stop()
}

func NewApp() (*App, error) {
	client, err := ifrit.NewClient()
	if err != nil {
		return nil, err
	}

	go client.Start()
	client.RegisterStreamHandler(streamHandler)

	master, err := ifrit.NewClient()
	if err != nil {
		return nil, err
	}

	go master.Start()
	return &App{
		master: master,
		client: client,
	}, nil
}

func (app *App) Stream() {
	var wg sync.WaitGroup
	inputStream, reply := app.master.OpenStream(app.client.Addr())

	// Test the input stream
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			msg := fmt.Sprintf("Message from client: %d", i)
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
		for i := 0; i < 10; i++ {
			msg := fmt.Sprintf("Server reply to client: hello from server %d", i)
			reply <- []byte(msg)
		}
		close(reply)
	}()

	wg.Wait()
}
