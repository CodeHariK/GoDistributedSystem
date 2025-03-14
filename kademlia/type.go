package kademlia

import (
	"container/list"
	"net"
	"net/http"
	"sync"

	"github.com/codeharik/kademlia/api/apiconnect"
)

const (
	CONST_K           = 20 // Number of contacts in each bucket.
	CONST_ALPHA       = 3  // Number of parallel queries at a time.
	CONST_TIMEOUT_SEC = 2  // RPC timeout duration.
)

// NodeID represents a 160-bit unique identifier.
type NodeID [20]byte

type Node struct {
	contact      Contact
	routingTable *RoutingTable

	httpClient  *http.Client
	connections map[NodeID]Connection
}

type Contact struct {
	ID NodeID
	IP net.TCPAddr
}

type KBucket struct {
	contacts list.List
	mutex    sync.Mutex
}

type RoutingTable struct {
	SelfID   NodeID
	Buckets  [160]*KBucket
	BucketMu sync.Mutex
}

type Connection struct {
	contact   Contact
	StartTime int64
	Client    apiconnect.KademliaClient
}
