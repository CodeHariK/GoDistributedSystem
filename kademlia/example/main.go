package main

import (
	"time"

	"github.com/codeharik/kademlia"
)

func main() {
	node := kademlia.NewNode()

	node.Start()

	time.Sleep(time.Second * 5)

	node.Shutdown()
}
