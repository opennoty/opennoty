package main

import (
	"fmt"
	"github.com/opennoty/opennoty/server"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	app, err := server.NewServer(nil)
	if err != nil {
		log.Fatalln(err)
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		fmt.Println(sig)
		app.Stop()
	}()

	if err := app.Start(); err != nil {
		log.Fatalln(err)
	}

	go func() {
		time.Sleep(time.Second * 3)
		app.PubSubService().Publish("", "hello", []byte("I AM SO HAPPY!!! 1"))
		app.PubSubService().Publish("", "hello", []byte("I AM SO HAPPY!!! 2"))
		app.PubSubService().Publish("", "hello", []byte("I AM SO HAPPY!!! 3"))
		app.PubSubService().Publish("", "hello", []byte("I AM SO HAPPY!!! 4"))
		app.PubSubService().Publish("", "hello", []byte("I AM SO HAPPY!!! 5"))
	}()

	if err := app.FiberServe(); err != nil {
		log.Fatalln(err)
	}
}
