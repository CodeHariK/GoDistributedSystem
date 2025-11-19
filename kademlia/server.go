package kademlia

import (
	"context"
	"errors"
	"fmt"

	"connectrpc.com/connect"
	"github.com/codeharik/kademlia/api"
)

func (node *Node) Ping(
	ctx context.Context, req *connect.Request[api.PingRequest]) (
	*connect.Response[api.PingResponse], error,
) {
	fmt.Printf("-> Ping : Node:%s\n", node.Key.HexString())

	return connect.NewResponse(&api.PingResponse{Status: "OK"}), nil
}

func (node *Node) GetContacts(
	ctx context.Context, req *connect.Request[api.GetContactsRequest]) (
	*connect.Response[api.GetContactsResponse], error,
) {
	fmt.Printf("-> GetContacts : Node:%s\n", node.Key.HexString())

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

func (node *Node) Join(
	ctx context.Context, req *connect.Request[api.JoinRequest]) (
	*connect.Response[api.JoinResponse], error,
) {
	fmt.Printf("-> Join : Node:%s (%v)\n", node.Key.HexString(), req.Msg.Self.Key)

	c, err := ToContact(req.Msg.Self)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kademlia.Kademlia.Join incorrect contact"))
	}

	contacts := node.FewContacts()

	var nodes []*api.Contact
	for _, contact := range contacts {
		apiContact, err := contact.ApiContact()
		if err == nil {
			nodes = append(nodes, apiContact)
		}
	}

	node.AddContact(c)

	cc, _ := node.ToContact()
	apic, _ := cc.ApiContact()
	return connect.NewResponse(&api.JoinResponse{
		Self:     apic,
		Contacts: nodes,
	}), nil
}

func (node *Node) FindNode(
	ctx context.Context, req *connect.Request[api.FindNodeRequest]) (
	*connect.Response[api.FindNodeResponse], error,
) {
	fmt.Printf("-> FindNode : Node:%s\n", node.Key.HexString())

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
	fmt.Printf("-> Store : Node:%s\n", node.Key.HexString())

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
	fmt.Printf("-> FindValue : Node:%s\n", node.Key.HexString())

	key, err := ToKKey(req.Msg.Key)
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("kademlia.Kademlia.FindValue Invalid Key"))
	}
	value, exists := node.kvStore.Get(key)
	if exists {
		return connect.NewResponse(&api.FindValueResponse{
			Value: value,
		}), nil
	}

	return nil, connect.NewError(connect.CodeNotFound, errors.New("kademlia.Kademlia.FindValue Not Found"))
}
