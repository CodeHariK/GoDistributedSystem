package kademlia

import (
	"container/list"
	"crypto/ecdsa"
	"net"
	"net/http"
	"sync"

	"github.com/codeharik/kademlia/api/apiconnect"
)

const (
	CONST_K           = 20 // Number of contacts in each bucket.
	CONST_ALPHA       = 3  // Number of parallel queries at a time.
	CONST_TIMEOUT_SEC = 2  // RPC timeout duration.

	CONST_KKEY_BIT_COUNT = 18 * 8
)

//	type TopicKKey struct {
//		Latitude    byte
//		Longitude   byte
//		TopicHash   [8]byte
//		ContentHash [8]byte
//	}
type KKey [18]byte

type Node struct {
	contact      Contact
	routingTable *RoutingTable

	listener net.Listener
	server   *http.Server

	httpClient  http.Client
	connections map[KKey]Connection

	kvStore KeyValueStore

	privateKey *ecdsa.PrivateKey

	quit chan any
	wg   sync.WaitGroup
	once sync.Once
}

type Contact struct {
	ID        KKey
	Addr      string
	PublicKey []byte
}

type KBucket struct {
	contacts list.List
	mutex    sync.Mutex
}

type RoutingTable struct {
	SelfID KKey

	Buckets [144]*KBucket

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
