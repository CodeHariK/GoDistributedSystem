package main

import (
	"log"
	"time"

	"github.com/codeharik/kademlia"
)

func main() {
	node, err := kademlia.NewNode("Hoopa")
	if err != nil {
		log.Fatal(err)
	}

	node.Start()

	time.Sleep(time.Second * 5)

	node.Shutdown()
}
