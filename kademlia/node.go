package kademlia

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/codeharik/kademlia/api"
	"github.com/codeharik/kademlia/api/apiconnect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func NewNode(domain, id string) (*Node, error) {
	domainKey, err := LoadKeyPair("TEMP/domain")
	if err != nil {
		return nil, err
	}
	idKey, err := LoadKeyPair("TEMP/id")
	if err != nil {
		return nil, err
	}
	nodeId, err := NewKKey(domain, id, nil, domainKey, idKey)
	if err != nil {
		return nil, err
	}

	transport := &http.Transport{}
	httpClient := http.Client{
		Transport: transport,
		Timeout:   100 * time.Millisecond,
	}

	node := Node{
		Key: nodeId,

		httpClient: httpClient,

		routingTable: NewRoutingTable(nodeId),

		kvStore: KeyValueStore{
			data: map[KKey][]byte{},
		},

		domain:    domain,
		domainKey: domainKey,
		id:        id,
		idKey:     idKey,

		quit: make(chan any),
	}

	// Create a TCP listener on a random available port, OS assigns a free port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle(apiconnect.NewKademliaHandler(&node))

	server := &http.Server{
		Addr: listener.Addr().String(), // Eg:"127.0.0.1:54321"
		Handler: h2c.NewHandler(
			mux,
			&http2.Server{},
		),
	}

	node.listener = listener
	node.server = server

	node.Addr = server.Addr

	return &node, nil
}

func (node *Node) Start() {
	node.wg.Add(1)
	defer node.wg.Done()

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	// Track if the server stops
	serverExited := make(chan struct{})

	go func() {
		log.Printf("[%s] Server listening at %s", node.Key.HexString(), node.Addr)
		if err := node.server.Serve(node.listener); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
		close(serverExited) // Signal that the server has stopped
	}()

	// Wait for signal
	select {
	case sig := <-sigChan:
		log.Printf("Received signal: %v. Shutting down...", sig)
	case <-node.quit:
		log.Printf("Received quit signal. Shutting down...")
	case <-serverExited:
		log.Printf("Server exited unexpectedly.")
	}
}

func (s *Node) Shutdown() {
	s.once.Do(func() { // Ensures this runs only once

		// Close quit channel only if this call initiated shutdown
		select {
		case <-s.quit:
			// Already closed, do nothing
		default:
			close(s.quit)
		}

		// Gracefully shut down the HTTP server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := s.server.Shutdown(ctx); err != nil {
			log.Printf("Shutdown error: %v", err)
			if err := s.server.Close(); err != nil {
				log.Fatalf("Server force close error: %v", err)
			}
		}

		if err := s.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("Listener close error: %v", err)
		}

		s.wg.Wait() // the program waits for all goroutines to exit

		log.Printf("Server terminated!")
	})
}

func (node *Node) GetClient(contact Contact) apiconnect.KademliaClient {
	if contact.Client != nil {
		return contact.Client
	}

	client := apiconnect.NewKademliaClient(
		&node.httpClient,
		"http://"+contact.Addr,
		// connect.WithGRPC(),
	)

	contact.Client = client
	return client
}

func (node *Node) CloseClient(nodeId KKey) {
}

type FindNodeResult struct {
	contact Contact
	path    []Contact
	err     error
}

func (node *Node) RFindNode(target KKey) FindNodeResult {
	return node.RecursiveFindNode(target, map[KKey]bool{}, []Contact{})
}

func (node *Node) RecursiveFindNode(target KKey, queried map[KKey]bool, path []Contact) FindNodeResult {
	// Get K closest nodes from the routing table
	closestNodes := node.FindClosest(target)

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
		if queried[contact.key] {
			continue // Skip already queried nodes
		}
		queried[contact.key] = true

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

				if contact.key == target {
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

// StartPingTicker runs a periodic ping to check active nodes.
func (node *Node) StartPingTicker() {
	ticker := time.NewTicker(30 * time.Second) // Adjust interval as needed
	defer ticker.Stop()

	for range ticker.C {
		node.PingOldestContacts()
	}
}

// PingOldestContacts pings the least recently seen nodes in each k-bucket.
func (node *Node) PingOldestContacts() {
	for _, bucket := range node.routingTable.Buckets {
		bucket.mu.Lock()
		if bucket.contacts.Len() == 0 {
			bucket.mu.Unlock()
			continue
		}

		// Get the oldest contact
		oldest := bucket.contacts.Peek()
		bucket.mu.Unlock()
		if oldest == nil {
			continue
		}

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		startTime := time.Now() // Record start time for RTT calculation

		res, err := node.GetClient(*oldest).Ping(ctx,
			connect.NewRequest(&api.PingRequest{}))

		if err != nil || res.Msg.Status != "OK" {
			log.Printf("Node %s unresponsive, removing from routing table\n", oldest.key)

			bucket.mu.Lock()
			bucket.contacts.Remove(oldest.key)
			bucket.mu.Unlock()
		} else {
			log.Printf("Node %s is active\n", oldest.key)

			oldest.RTT = int32(time.Since(startTime).Milliseconds())
		}
	}
}
