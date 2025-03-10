package craft

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/rpc"

	"connectrpc.com/connect"
	"github.com/codeharik/craft/api/apiconnect"
)

func NewServer(serverId int64, peerIds []int64, ready <-chan any) *CraftServer {
	return &CraftServer{
		serverID:    serverId,
		peerIDs:     peerIds,
		peerClients: make(map[int]*apiconnect.CraftServiceClient),
		ready:       ready,
		quit:        make(chan any),
	}
}

func (s *CraftServer) Serve() {
	s.mu.Lock()
	s.cm = NewConsensusModule(s.serverID, s.peerIDs, s, s.ready)

	// Create a new RPC server and register a RPCProxy that forwards all methods
	// to n.cm
	s.rpcServer = rpc.NewServer()
	s.rpcProxy = &RPCProxy{cm: s.cm}
	s.rpcServer.RegisterName("ConsensusModule", s.rpcProxy)

	var err error
	s.listener, err = net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("[%v] listening at %s", s.serverID, s.listener.Addr())
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		for {
			conn, err := s.listener.Accept()
			if err != nil {
				select {
				case <-s.quit:
					return
				default:
					log.Fatal("accept error:", err)
				}
			}
			s.wg.Add(1)
			go func() {
				s.rpcServer.ServeConn(conn)
				s.wg.Done()
			}()
		}
	}()
}

// DisconnectAll closes all the client connections to peers for this server.
func (s *CraftServer) DisconnectAll() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for id := range s.peerClients {
		if conn, exists := s.peerClients[id]; exists && conn != nil {
			conn.http.CloseIdleConnections()
			delete(s.peerClients, id)
		}
	}
}

// Shutdown closes the server and waits for it to shut down properly.
func (s *CraftServer) Shutdown() {
	s.cm.Stop()
	close(s.quit)
	s.listener.Close()
	s.wg.Wait()
}

func (s *CraftServer) GetListenAddr() net.Addr {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.listener.Addr()
}

func (s *CraftServer) ConnectToPeer(peerId int64, addr string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.peerClients[peerId] == nil {

		// Create a persistent HTTP client
		transport := &http.Transport{} // Customize if needed
		httpClient := &http.Client{Transport: transport}

		// Create a persistent ConnectRPC client
		client := apiconnect.NewCraftServiceClient(httpClient, "http://"+addr, connect.WithGRPC()) // Use gRPC for persistence

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

func (s *CraftServer) Call(id int64, serviceMethod string, args any, reply any) error {
	s.mu.Lock()
	peer := s.peerClients[id]
	s.mu.Unlock()

	// If this is called after shutdown (where client.Close is called), it will
	// return an error.
	if peer == nil {
		return fmt.Errorf("call client %d after it's closed", id)
	} else {
		return peer.Call(serviceMethod, args, reply)
	}
}
