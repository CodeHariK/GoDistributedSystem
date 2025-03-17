package kademlia

import (
	"context"
	"fmt"
	"testing"
	"time"

	"connectrpc.com/connect"
	"github.com/codeharik/kademlia/api"
	"github.com/codeharik/kademlia/api/apiconnect"
)

// TestHarness initializes multiple nodes, connects them, and verifies operations.
func TestHarness(t *testing.T) {
	bootstrap, err := NewNode("BootDomain", "BootId")
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
		n, err := NewNode(fmt.Sprint("NodeDomain", i), fmt.Sprint("NodeId", i))
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

		c, err := nodes[i].ToContact()
		if err != nil {
			continue
		}
		cc, err := c.ApiContact()
		if err != nil {
			continue
		}

		res, err := client.Join(context.Background(),
			connect.NewRequest(&api.JoinRequest{Self: cc}))

		if err != nil {
			t.Errorf("Node %d failed to join Network[%s]: %v\n", i, "http://"+bootstrap.Addr, err)
		} else {
			fmt.Printf("B:%s\nN:%s\nL:%d\n", bootstrap.Key.BitString(), nodes[i].Key.BitString(), len(res.Msg.Contacts))
		}

		time.Sleep(1 * time.Second) // Wait for nodes to start
	}

	time.Sleep(2 * time.Second) // Wait for nodes to start

	if bootstrap.NumContacts() == 0 {
		t.Errorf("Node bootstrap has an empty routing table")
	}
	for i, node := range nodes {
		if node.NumContacts() == 0 {
			t.Errorf("Node %d has an empty routing table", i)
		}
	}

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
