package gossip

import (
	"context"
	"fmt"
	"math/rand/v2"

	"connectrpc.com/connect"
	"github.com/codeharik/gossip/api"
)

func (node *GossipNode) BroadcastMessage(
	ctx context.Context,
	req *connect.Request[api.BroadcastRequest],
) (*connect.Response[api.BroadcastResponse], error) {
	// Log message
	fmt.Printf("[%d] Received message from %d: %s\n",
		node.ID, req.Msg.OriginId, req.Msg.Message)

	// Forward the message with decremented hop count
	msg := req.Msg

	node.PeersLock.Lock()
	defer node.PeersLock.Unlock()

	if len(node.Peers) == 0 {
		return connect.NewResponse(&api.BroadcastResponse{Received: true}), nil
	}

	// Send new message with decremented hop count
	broadcastMsg := &api.BroadcastRequest{
		OriginId:    node.ID,
		MessageType: msg.MessageType,
		Message:     msg.Message,
		Timestamp:   msg.Timestamp,
	}

	{
		peerList := make([]GossipNodeInfo, 0, len(node.Peers))

		for _, peer := range node.Peers {
			peerList = append(peerList, peer)
		}

		n := node.BROADCAST_COUNT
		if n > len(peerList) {
			n = len(peerList)
		}

		// Shuffle the slice
		rand.Shuffle(len(peerList), func(i, j int) {
			peerList[i], peerList[j] = peerList[j], peerList[i]
		})

		minPeerList := peerList[:n]
		for _, nodeInfo := range minPeerList {
			_, err := nodeInfo.connection.BroadcastMessage(
				context.Background(), connect.NewRequest(broadcastMsg))
			if err != nil {
				fmt.Printf("Failed to forward gossip to %v: %v\n", nodeInfo, err)
			}
		}
	}
	return connect.NewResponse(&api.BroadcastResponse{Received: true}), nil
}

func (node *GossipNode) Connect(ctx context.Context, req *connect.Request[api.ConnectRequest],
) (*connect.Response[api.ConnectResponse], error) {
	node.PeersLock.Lock()
	defer node.PeersLock.Unlock()

	peerList := make([]*api.Peer, 0, len(node.Peers))
	for _, peer := range node.Peers {
		peerList = append(peerList, &api.Peer{
			Addr:      peer.Addr,
			StartTime: peer.StartTime,
		})
	}

	if !IsBootstrap(node.ID) {
		node.Peers[req.Msg.Peer.PeerId] = GossipNodeInfo{
			ID:         req.Msg.Peer.PeerId,
			Addr:       req.Msg.Peer.Addr,
			StartTime:  req.Msg.Peer.StartTime,
			connection: createGossipConnection(req.Msg.Peer.Addr),
		}
	}

	fmt.Println("Connect-> ", req.Msg.Peer)

	return connect.NewResponse(&api.ConnectResponse{KnownPeers: peerList}), nil
}
