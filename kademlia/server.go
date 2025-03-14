package kademlia

import (
	"context"
	"errors"
	"log"

	"connectrpc.com/connect"
	"github.com/codeharik/kademlia/api"
)

func (node *Node) Ping(ctx context.Context, req *connect.Request[api.PingRequest]) (*connect.Response[api.PingResponse], error) {
	log.Println("Received Ping from:", req.Msg.NodeId)
	return connect.NewResponse(&api.PingResponse{Status: "OK"}), nil
}

func (node *Node) FindNode(ctx context.Context, req *connect.Request[api.FindNodeRequest]) (*connect.Response[api.FindNodeResponse], error) {
	targetId, err := NodeIDFromKKey(req.Msg.TargetId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("kademlia.Kademlia.FindNode incorrect targetID"))
	}

	closestNodes := node.routingTable.FindClosest(targetId, CONST_K)

	var nodes []*api.Contact
	for _, contact := range closestNodes {

		nodeId, err := contact.ID.KKey()
		if err != nil {
			nodes = append(nodes, &api.Contact{
				NodeId: nodeId,
				Ip:     contact.IP.String(),
				Port:   int32(contact.IP.Port),
			})
		}
	}

	return connect.NewResponse(&api.FindNodeResponse{Contacts: nodes}), nil
}

func (node *Node) Store(context.Context, *connect.Request[api.StoreRequest]) (*connect.Response[api.StoreResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("kademlia.Kademlia.Store is not implemented"))
}

func (node *Node) FindValue(context.Context, *connect.Request[api.FindValueRequest]) (*connect.Response[api.FindValueResponse], error) {
	return nil, connect.NewError(connect.CodeUnimplemented, errors.New("kademlia.Kademlia.FindValue is not implemented"))
}
