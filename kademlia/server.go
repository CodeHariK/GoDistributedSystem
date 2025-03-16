package kademlia

import (
	"context"
	"errors"
	"log"

	"connectrpc.com/connect"
	"github.com/codeharik/kademlia/api"
)

func (node *Node) Ping(
	ctx context.Context, req *connect.Request[api.PingRequest]) (
	*connect.Response[api.PingResponse], error,
) {
	log.Println("Received Ping from:", req.Msg.Contact.NodeId)
	return connect.NewResponse(&api.PingResponse{Status: "OK"}), nil
}

func (node *Node) GetContacts(
	ctx context.Context, req *connect.Request[api.GetContactsRequest]) (
	*connect.Response[api.GetContactsResponse], error,
) {
	targetId, err := ToKKey(req.Msg.TargetId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kademlia.Kademlia.FindNode incorrect targetID"))
	}

	closestNodes := node.FindClosest(targetId)

	var nodes []*api.Contact
	for _, contact := range closestNodes {
		apiContact, err := contact.ApiContact()
		if err != nil {
			nodes = append(nodes, apiContact)
		}
	}

	return connect.NewResponse(&api.GetContactsResponse{Contacts: nodes}), nil
}

func (node *Node) FindNode(
	ctx context.Context, req *connect.Request[api.FindNodeRequest]) (
	*connect.Response[api.FindNodeResponse], error,
) {
	targetId, err := ToKKey(req.Msg.TargetId)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kademlia.Kademlia.FindNode incorrect targetID"))
	}

	findNodeResult := node.RFindNode(targetId)
	if findNodeResult.err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("kademlia.Kademlia.FindNode not found"))
	}

	apiContact, err := findNodeResult.contact.ApiContact()
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kademlia.Kademlia.FindNode incorrect contact"))
	}

	pathContact := make([]*api.Contact, len(findNodeResult.path))
	for i, c := range findNodeResult.path {
		cc, err := c.ApiContact()
		if err == nil {
			pathContact[i] = cc
		}
	}

	return connect.NewResponse(&api.FindNodeResponse{
		Contact: apiContact,
		Path:    pathContact,
	}), nil
}

func (node *Node) Store(
	ctx context.Context, req *connect.Request[api.StoreRequest]) (
	*connect.Response[api.StoreResponse], error,
) {
	key, err := ToKKey(req.Msg.Key)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kademlia.Kademlia.Store Invalid Key"))
	}
	node.kvStore.Store(key, req.Msg.Value)
	return connect.NewResponse(&api.StoreResponse{Status: "OK"}), nil
}

func (node *Node) FindValue(
	ctx context.Context, req *connect.Request[api.FindValueRequest]) (
	*connect.Response[api.FindValueResponse], error,
) {
	key, err := ToKKey(req.Msg.Key)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kademlia.Kademlia.FindValue Invalid Key"))
	}
	value, exists := node.kvStore.Get(key)
	if exists {
		return connect.NewResponse(&api.FindValueResponse{
			Response: &api.FindValueResponse_Value{Value: value},
		}), nil
	}

	closestNodes := node.FindClosest(key)

	var nodes []*api.Contact
	for _, contact := range closestNodes {
		apiContact, err := contact.ApiContact()
		if err != nil {
			nodes = append(nodes, apiContact)
		}
	}

	return connect.NewResponse(&api.FindValueResponse{
		Response: &api.FindValueResponse_Nodes{
			Nodes: &api.GetContactsResponse{Contacts: nodes},
		},
	}), nil
}
