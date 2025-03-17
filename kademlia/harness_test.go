package kademlia

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/codeharik/kademlia/api"
	"github.com/codeharik/kademlia/api/apiconnect"
)

// TestHarness initializes multiple nodes, connects them, and verifies operations.
func TestHarness(t *testing.T) {
	hello := make([]byte, 8)
	rand.Read(hello)

	bootstrap, err := NewNode(string(hello), string(hello))
	if err != nil {
		t.Fatal(err)
	}
	go func() {
		bootstrap.Start()
	}()

	numNodes := 2
	var nodes []*Node

	// Create and start nodes
	for i := 0; i < numNodes; i++ {
		rand.Read(hello)
		n, err := NewNode(string(hello), string(hello))
		if err == nil {
			nodes = append(nodes, n)

			go func(node *Node) {
				node.Start()
			}(n)
		}
	}
	time.Sleep(2 * time.Second) // Wait for nodes to start

	for i := 0; i < numNodes; i++ {
		client := apiconnect.NewKademliaClient(
			&nodes[i].httpClient,
			"http://"+bootstrap.Addr,
			// connect.WithGRPC(),
		)

		// c, err := nodes[i].ToContact()
		// if err != nil {
		// 	continue
		// }
		// cc, err := c.ApiContact()
		// if err != nil {
		// 	continue
		// }

		req := api.JoinRequest{
			// Self:  cc,
			Hello: ".....Hello I am.....",
		}
		b, _ := json.MarshalIndent(req, "", " ")
		fmt.Println("---> ", string(b))

		_, err := client.Join(context.Background(),
			connect.NewRequest(&req))
		if err != nil {
			t.Errorf("Node %d failed to join Network[%s]: %v\n", i, "http://"+bootstrap.Addr, err)
		} else {
			// fmt.Printf("\nB:%s\nN:%s\nV:%v\nC:%v\n", bootstrap.Key.BitString(), nodes[i].Key.BitString()) // res.Msg.Self, res.Msg.Contacts)
		}

	}

	// time.Sleep(2 * time.Second) // Wait for nodes to start

	// // if bootstrap.NumContacts() == 0 {
	// // 	t.Errorf("Node bootstrap has an empty routing table")
	// // }
	// // for i, node := range nodes {
	// // 	if node.NumContacts() == 0 {
	// // 		t.Errorf("Node %d has an empty routing table", i)
	// // 	}
	// // }

	// // // Perform a store operation on Node 0
	// // key := KKey{1, 2, 3} // Sample key
	// // value := []byte("Hello, Kademlia!")
	// // nodes[0].kvStore.Store(key, value)

	// // // Verify retrieval from other nodes
	// // for i := 1; i < numNodes; i++ {
	// // 	retrieved, err := nodes[i].kvStore.Get(key)
	// // 	if err != nil || string(retrieved) != string(value) {
	// // 		t.Errorf("Node %d failed to retrieve stored value", i)
	// // 	}
	// // }

	// // Stop all nodes
	// for _, node := range nodes {
	// 	node.Shutdown()
	// }

	bootstrap.Shutdown()
}
