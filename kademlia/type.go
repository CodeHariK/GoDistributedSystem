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

	CONST_KKEY_BIT_COUNT = 64 // 160-bit standard, 64 for testing
)

// KKey represents a 160-bit unique identifier.
// type KKey [20]byte
type KKey [8]byte

type Node struct {
	contact      Contact
	routingTable *RoutingTable

	listener net.Listener
	server   *http.Server

	httpClient  http.Client
	connections map[KKey]Connection

	kvStore KeyValueStore

	quit chan any
	wg   sync.WaitGroup
	once sync.Once
}

type Contact struct {
	ID   KKey
	Addr string
}

type KBucket struct {
	contacts list.List
	mutex    sync.Mutex
}

type RoutingTable struct {
	SelfID KKey

	// Buckets  [160]*KBucket
	Buckets [64]*KBucket

	BucketMu sync.Mutex
}

type Connection struct {
	StartTime int64
	RTT       int32
	Client    apiconnect.KademliaClient
}

type KeyValueStore struct {
	data map[KKey][]byte
	mu   sync.RWMutex
}
