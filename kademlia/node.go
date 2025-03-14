package kademlia

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/bits"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"

	"connectrpc.com/connect"
	"github.com/codeharik/kademlia/api"
	"github.com/codeharik/kademlia/api/apiconnect"
)

// Address returns the full network address of the contact.
func (c Contact) Address() string {
	return fmt.Sprint(c.IP.String(), ":", c.IP.Port)
}

// NewNodeID generates a random NodeID.
func NewNodeID() NodeID {
	var id NodeID
	rand.Read(id[:])
	return id
}

func (id NodeID) KKey() (*api.KKey, error) {
	key := api.KKey{Key: id[:]}
	return &key, nil
}

func NodeIDFromKKey(s *api.KKey) (NodeID, error) {
	if len(s.Key) != 20 {
		return NodeID{}, errors.New("Key length not 20")
	}
	var id NodeID
	copy(id[:], s.Key)
	return id, nil
}

func NewNode() Node {
	transport := &http.Transport{}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   100 * time.Millisecond,
	}

	node := Node{
		httpClient:  httpClient,
		connections: make(map[NodeID]Connection),
	}

	return node
}

// Distance calculates XOR distance between two NodeIDs.
func (id NodeID) Distance(other NodeID) NodeID {
	var dist NodeID
	for i := 0; i < len(id); i++ {
		dist[i] = id[i] ^ other[i]
	}
	return dist
}

// LeadingZeros returns the index of the first nonzero bit.
func (id NodeID) LeadingZeros() int {
	for i := 0; i < len(id); i++ {
		if id[i] != 0 {
			return i*8 + bits.LeadingZeros8(id[i])
		}
	}
	return 160 // All zero case (extremely rare)
}

func (node *Node) GetClient(contact Contact) Connection {
	if client, exists := node.connections[contact.ID]; exists {
		return client
	}

	client := apiconnect.NewKademliaClient(
		node.httpClient, node.contact.Address(),
		// connect.WithGRPC(),
	)

	// Create new client if not cached
	node.connections[contact.ID] = Connection{contact: contact, Client: client}
	return node.connections[contact.ID]
}

func (node *Node) CloseClient(nodeId NodeID) {
	if _, exists := node.connections[nodeId]; exists {
		delete(node.connections, nodeId)
	}
}

// IterativeFindNode performs iterative `FindNode` lookup.
//  1. Starts with K closest nodes from the routing table.
//  2. Performs parallel queries (Alpha at a time):
//     •	Sends FindNode RPC requests.
//     •	Waits for responses.
//     •	Adds new contacts to the queue.
//  3. Sorts nodes by XOR distance and continues.
//  4. Stops when no closer nodes are found.
func (node *Node) IterativeFindNode(targetID NodeID) []Contact {
	// Step 1: Start with K closest known nodes from the routing table.
	closestNodes := node.routingTable.FindClosest(targetID, CONST_K)
	visited := make(map[NodeID]bool)
	result := make([]Contact, 0)

	// Priority queue sorted by XOR distance.
	var queue []Contact
	queue = append(queue, closestNodes...)

	var mu sync.Mutex
	var wg sync.WaitGroup

	// Step 2: Perform parallel lookups (Alpha nodes at a time).
	for len(queue) > 0 {
		// Pick up to `Alpha` closest unvisited nodes.
		parallelNodes := make([]Contact, 0, CONST_ALPHA)
		for i := 0; i < len(queue) && len(parallelNodes) < CONST_ALPHA; i++ {
			if !visited[queue[i].ID] {
				visited[queue[i].ID] = true
				parallelNodes = append(parallelNodes, queue[i])
			}
		}

		// No new nodes to query
		if len(parallelNodes) == 0 {
			break
		}

		wg.Add(len(parallelNodes))
		for _, contact := range parallelNodes {
			go func(contact Contact) {
				defer wg.Done()

				ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
				defer cancel()

				targetID, err := targetID.KKey()
				if err != nil {
					res, err := node.GetClient(contact).Client.FindNode(ctx,
						connect.NewRequest(&api.FindNodeRequest{
							TargetId: targetID,
						}))
					if err != nil {
						log.Println("Error contacting", contact.Address(), err)
						return
					}

					mu.Lock()
					for _, c := range res.Msg.Contacts {
						nodeId, err := NodeIDFromKKey(c.NodeId)
						if err == nil && !visited[nodeId] {
							queue = append(queue, Contact{
								ID: nodeId,
								IP: net.TCPAddr{
									IP:   net.IP(c.Ip),
									Port: int(c.Port),
								},
							})
						}
					}
					mu.Unlock()
				}
			}(contact)
		}
		wg.Wait()

		// Step 3: Sort queue by XOR distance
		sort.Slice(queue, func(i, j int) bool {
			return targetID.Distance(queue[i].ID).LeadingZeros() > targetID.Distance(queue[j].ID).LeadingZeros()
		})

		// Step 4: Keep only the closest `K` nodes.
		if len(queue) > CONST_K {
			queue = queue[:CONST_K]
		}

		result = queue
	}

	return result
}
