package kademlia

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/bits"
	"net"
	"net/http"
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
func NewNodeID() KKey {
	var id KKey
	rand.Read(id[:])
	return id
}

func (id KKey) ApiKKey() (*api.KKey, error) {
	if len(id) != 20 {
		return nil, errors.New("Key length not 20")
	}
	key := api.KKey{Key: id[:]}
	return &key, nil
}

func ToKKey(s *api.KKey) (KKey, error) {
	if len(s.Key) != 20 {
		return KKey{}, errors.New("Key length not 20")
	}
	var id KKey
	copy(id[:], s.Key)
	return id, nil
}

func (contact Contact) ApiContact() (*api.Contact, error) {
	contactKey, err := contact.ID.ApiKKey()
	if err != nil {
		return nil, err
	}
	return &api.Contact{
		NodeId: contactKey,
		Ip:     contact.IP.String(),
		Port:   int32(contact.IP.Port),
	}, nil
}

func ToContact(s *api.Contact) (Contact, error) {
	contactKey, err := ToKKey(s.NodeId)
	if err != nil {
		return Contact{}, err
	}
	return Contact{
		ID: contactKey,
		IP: net.TCPAddr{
			IP:   net.IP(s.Ip),
			Port: int(s.Port),
		},
	}, nil
}

func NewNode() *Node {
	transport := &http.Transport{}
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   100 * time.Millisecond,
	}

	node := Node{
		httpClient:  httpClient,
		connections: make(map[KKey]Connection),

		kvStore: KeyValueStore{
			data: map[KKey][]byte{},
		},
	}

	return &node
}

// Distance calculates XOR distance between two NodeIDs.
func (id KKey) Distance(other KKey) KKey {
	var dist KKey
	for i := 0; i < len(id); i++ {
		dist[i] = id[i] ^ other[i]
	}
	return dist
}

// LeadingZeros returns the index of the first nonzero bit.
func (id KKey) LeadingZeros() int {
	for i := 0; i < len(id); i++ {
		if id[i] != 0 {
			return i*8 + bits.LeadingZeros8(id[i])
		}
	}
	return 160 // All zero case (extremely rare)
}

func (node *Node) GetClient(contact Contact) apiconnect.KademliaClient {
	if client, exists := node.connections[contact.ID]; exists {
		return client.Client
	}

	client := apiconnect.NewKademliaClient(
		node.httpClient,
		node.contact.Address(),
		// connect.WithGRPC(),
	)

	// Create new client if not cached
	node.connections[contact.ID] = Connection{contact: contact, Client: client}
	return node.connections[contact.ID].Client
}

func (node *Node) CloseClient(nodeId KKey) {
	if _, exists := node.connections[nodeId]; exists {
		delete(node.connections, nodeId)
	}
}

func (node *Node) FindClosest(target KKey) []Contact {
	return node.routingTable.FindClosest(target)
}

func (node *Node) RFindNode(target KKey) FindNodeResult {
	return node.RecursiveFindNode(target, map[KKey]bool{}, []Contact{})
}

type FindNodeResult struct {
	contact Contact
	path    []Contact
	err     error
}

func (node *Node) RecursiveFindNode(target KKey, queried map[KKey]bool, path []Contact) FindNodeResult {
	// Get K closest nodes from the routing table
	closestNodes := node.routingTable.FindClosest(target)

	// Base case: If no new nodes to query, return error
	if len(closestNodes) == 0 {
		return FindNodeResult{Contact{}, path, errors.New("target node not found")}
	}

	var wg sync.WaitGroup
	found := make(chan Contact, 1) // Channel for early termination
	results := make(chan struct {  // Channel for recursive results
		contact Contact
		path    []Contact
		err     error
	}, len(closestNodes))

	for i := 0; i < len(closestNodes) && i < CONST_ALPHA; i++ {
		contact := closestNodes[i]
		if queried[contact.ID] {
			continue // Skip already queried nodes
		}
		queried[contact.ID] = true

		wg.Add(1)
		go func(contact Contact, localPath []Contact) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
			defer cancel()

			targetApiKKey, err := target.ApiKKey()
			if err != nil {
				results <- FindNodeResult{Contact{}, localPath, err}
				return
			}

			res, err := node.GetClient(contact).GetContacts(ctx,
				connect.NewRequest(&api.GetContactsRequest{TargetId: targetApiKKey}))
			if err != nil {
				results <- FindNodeResult{Contact{}, localPath, err}
				return
			}

			// Extend path for this branch
			localPath = append(localPath, contact)

			// Check if target node is found
			for _, c := range res.Msg.Contacts {
				contact, err := ToContact(c)
				if err != nil {
					continue
				}

				if contact.ID == target {
					found <- contact
					results <- FindNodeResult{contact, localPath, nil}
					return
				}
			}

			// Recursive step: Pass unique path for each query
			findNodeResult := node.RecursiveFindNode(target, queried, localPath)
			results <- findNodeResult
		}(contact, append([]Contact{}, path...)) // Ensure unique path for each lookup
	}

	// Wait for results
	go func() {
		wg.Wait()
		close(found)
		close(results)
	}()

	// If a node finds the target, return immediately
	if target, ok := <-found; ok {
		return FindNodeResult{target, path, nil}
	}

	// Check recursive results for the best path
	for res := range results {
		if res.err == nil {
			return FindNodeResult{res.contact, res.path, nil}
		}
	}

	return FindNodeResult{Contact{}, path, errors.New("target node not found")}
}
