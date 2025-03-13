package gossip

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"connectrpc.com/connect"
	"golang.org/x/net/http2"
	"golang.org/x/net/http2/h2c"

	"github.com/codeharik/gossip/api"
	"github.com/codeharik/gossip/api/apiconnect"
)

func IsBootstrap(id uint64) bool {
	return (id & BOOTSTRAP) > 0
}

func createGossipConnection(addr string) apiconnect.GossipServiceClient {
	return apiconnect.NewGossipServiceClient(http.DefaultClient, "http://"+addr)
}

func JoinGossip(port int, BootstrapAddr string) (*GossipNode, error) {
	BootstrapConnection := createGossipConnection(BootstrapAddr)

	res, err := BootstrapConnection.Connect(
		context.Background(), connect.NewRequest(&api.ConnectRequest{}))
	if err != nil {
		return nil, errors.New("Bootstrap connection failure")
	}

	newNodes := make(map[uint64]GossipNodeInfo)
	for _, peer := range res.Msg.KnownPeers {
		newNodes[peer.PeerId] = GossipNodeInfo{
			ID:         peer.PeerId,
			Addr:       peer.Addr,
			StartTime:  peer.StartTime,
			connection: createGossipConnection(peer.Addr),
		}
	}

	node := GossipNode{
		ID:   res.Msg.ApproxNumNode + uint64(time.Now().Unix()),
		Addr: fmt.Sprintf("localhost:%d", port),

		APPROX_NUM_NODE: res.Msg.ApproxNumNode,

		MIN_PEER:        10,
		MAX_PEER:        20,
		BROADCAST_COUNT: 10,

		Bootstraps: map[uint64]GossipNodeInfo{
			res.Msg.Peer.PeerId: {
				ID:         res.Msg.Peer.PeerId,
				Addr:       res.Msg.Peer.Addr,
				StartTime:  res.Msg.Peer.StartTime,
				connection: BootstrapConnection,
			},
		},
		BootstrapsLock: sync.Mutex{},

		Peers:     newNodes,
		PeersLock: sync.Mutex{},
	}

	return &node, nil
}

func (node *GossipNode) StartGossipLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for range ticker.C {
		node.PeersLock.Lock()
		node.PeersLock.Unlock()

		activeCount := 0
		approxNumNode := uint64(0)

		for _, peer := range node.Peers {
			go func() {
				msg, err := peer.connection.Connect(
					context.Background(), connect.NewRequest(&api.ConnectRequest{
						Peer: &api.Peer{
							Addr: node.Addr,
						},
					}))
				if err != nil {
					fmt.Printf("[%d] Peer %s is unresponsive, removing...\n", node.ID, peer.Addr)
					delete(node.Peers, peer.ID)
				} else {

					approxNumNode += msg.Msg.ApproxNumNode
					activeCount++

					for _, peer := range msg.Msg.KnownPeers {
						node.Peers[peer.PeerId] = GossipNodeInfo{
							ID:         peer.PeerId,
							Addr:       peer.Addr,
							StartTime:  peer.StartTime,
							connection: createGossipConnection(peer.Addr),
						}
					}
				}
			}()
		}

		node.APPROX_NUM_NODE = (approxNumNode + node.APPROX_NUM_NODE) / uint64(activeCount+1)

	}
}

func Gossip(port int) {
	node, err := JoinGossip(port, "localhost:8888")
	if err != nil {
		panic("The END!")
	}

	// Start gossip loop in the background
	go node.StartGossipLoop()

	// HTTP server setup
	mux := http.NewServeMux()
	mux.Handle(apiconnect.NewGossipServiceHandler(node))

	server := &http.Server{
		Addr:    node.Addr,
		Handler: h2c.NewHandler(mux, &http2.Server{}),
	}

	// Handle OS signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Println("Node running on ", port)
		if err := server.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for signal
	sig := <-sigChan
	log.Printf("Received signal: %v. Shutting down...", sig)

	// Graceful shutdown
	if err := server.Shutdown(context.Background()); err != nil {
		log.Fatalf("Server shutdown error: %v", err)
	}

	log.Println("Node terminated")
}
