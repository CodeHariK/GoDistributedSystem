package craft

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"github.com/codeharik/craft/api/apiconnect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"
)

func NewServer(serverId int64, peerIds []int64, storage Storage, ready <-chan any, commitChan chan<- CommitEntry) *CraftServer {
	return &CraftServer{
		serverID:    serverId,
		peerIDs:     peerIds,
		peerClients: make(map[int64]*Connection),
		storage:     storage,
		commitChan:  commitChan,
		ready:       ready,
		quit:        make(chan any),
	}
}

func (s *CraftServer) Serve() {
	s.mu.Lock()
	s.cm = NewConsensusModule(s.serverID, s.peerIDs, s, s.storage, s.ready, s.commitChan)

	// Create a new RPC server and register a RPCProxy that forwards all methods to s.cm
	s.rpcProxy = &RPCProxy{cm: s.cm}

	// Create a TCP listener on a random available port
	listener, err := net.Listen("tcp", ":0") // OS assigns a free port
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	s.server.listener = listener

	mux := http.NewServeMux()
	mux.Handle(apiconnect.NewCraftServiceHandler(s.cm))

	s.server.server = http.Server{
		Addr: listener.Addr().String(), // Get assigned address (e.g., "127.0.0.1:54321")
		Handler: h2c.NewHandler(
			mux,
			&http2.Server{},
		),
	}

	s.cm.dlog("listening at %s", s.server.server.Addr)
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		// Handle OS signals for graceful shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// Track if the server stops
		serverExited := make(chan struct{})

		go func() {
			if err := s.server.server.Serve(listener); err != http.ErrServerClosed {
				log.Fatalf("[%d] Server error: %v", s.cm.id, err)
			}
			close(serverExited) // Signal that the server has stopped
		}()

		// Wait for signal
		select {
		case sig := <-sigChan:
			log.Printf("[%d] Received signal: %v. Shutting down...", s.cm.id, sig)
		case <-s.quit:
			log.Printf("[%d] Received quit signal. Shutting down...", s.cm.id)
		case <-serverExited:
			log.Printf("[%d] Server exited unexpectedly.", s.cm.id)
		}
	}()
}

// Submit wraps the underlying CM's Submit; see that method for documentation.
func (s *CraftServer) Submit(cmd int64) int64 {
	return s.cm.Submit(cmd)
}

// Shutdown closes the server and waits for it to shut down properly.
func (s *CraftServer) Shutdown() {
	s.once.Do(func() { // Ensures this runs only once
		s.cm.Stop()

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

		if err := s.server.server.Shutdown(ctx); err != nil {
			s.cm.dlog("Shutdown error: %v", err)
			if err := s.server.server.Close(); err != nil {
				log.Fatalf("Server force close error: %v", err)
			}
		}

		if err := s.server.listener.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
			log.Printf("[%d] Listener close error: %v", s.cm.id, err)
		}

		s.wg.Wait() // the program waits for all goroutines to exit

		s.cm.dlog("CraftServer terminated!")
	})
}

func (s *CraftServer) GetListenAddr() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.server.server.Addr
}

func (s *CraftServer) GetPeerClient(peerId int64) (*Connection, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Unlock before making network calls
	// dont blocks other goroutines from accessing cm.server.peerClients
	peer, exists := s.peerClients[peerId]
	return peer, exists
}

func (s *CraftServer) ConnectToPeer(peerId int64, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.peerClients[peerId] == nil {

		// Create a persistent HTTP client
		transport := &http.Transport{}
		httpClient := &http.Client{Transport: transport}

		// Create a persistent ConnectRPC client, Use gRPC for persistence
		client := apiconnect.NewCraftServiceClient(
			httpClient, "http://"+addr, connect.WithGRPC())

		s.peerClients[peerId] = &Connection{
			http:   httpClient,
			client: client,
		}
	}
	return nil
}

// DisconnectPeer disconnects this server from the peer identified by peerId.
func (s *CraftServer) DisconnectPeer(peerId int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if conn, exists := s.peerClients[peerId]; exists && conn != nil {
		conn.http.CloseIdleConnections()
		s.peerClients[peerId] = nil
	}
}

// DisconnectAll closes all the client connections to peers for this server.
func (s *CraftServer) DisconnectAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.peerClients {
		if conn, exists := s.peerClients[id]; exists && conn != nil {
			conn.http.CloseIdleConnections()
			// delete(s.peerClients, id)
			s.peerClients[id] = nil
		}
	}
}

// IsLeader checks if s thinks it's the leader in the Raft cluster.
func (s *CraftServer) IsLeader() bool {
	_, _, isLeader := s.cm.Report()
	return isLeader
}
