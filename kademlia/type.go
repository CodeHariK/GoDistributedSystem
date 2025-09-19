package kademlia

import (
	"crypto/ecdsa"
	"net"
	"net/http"
	"sync"

	"github.com/codeharik/kademlia/api/apiconnect"
)

const (
	CONST_K           = 20 // Number of contacts in each bucket.
	CONST_ALPHA       = 5  // Number of parallel queries at a time.
	CONST_TIMEOUT_SEC = 2  // RPC timeout duration.

	CONST_KKEY_BYTE_COUNT = 18
	CONST_KKEY_BIT_COUNT  = CONST_KKEY_BYTE_COUNT * 8
)

//	type DomainKKey struct {
//		DomainHash  [8]byte
//		Latitude    byte
//		Longitude   byte
//		ContentHash [8]byte
//	}
type KKey [18]byte

type Node struct {
	Key         KKey
	SERVER_MODE bool // Is this node a CLIENT or SERVER

	routingTable *RoutingTable

	listener net.Listener
	server   *http.Server

	httpClient http.Client

	topics []string

	kvStore KeyValueStore

	domain    string
	domainKey *ecdsa.PrivateKey
	id        string
	idKey     *ecdsa.PrivateKey

	quit chan any
	wg   sync.WaitGroup
	once sync.Once
}

type Contact struct {
	key  KKey
	Addr string

	domain    string
	domainKey *ecdsa.PublicKey
	id        string
	idKey     *ecdsa.PublicKey

	StartTime int64
	RTT       int32
	Client    apiconnect.KademliaClient

	SERVER_MODE bool // Is this node a CLIENT or SERVER
}

// ContactHeap implements a min-heap based on StartTime and RTT.
type (
	ContactHeap []Contact
	KBucket     struct {
		contacts ContactHeap
		mu       sync.Mutex
	}
	RoutingTable struct {
		Buckets [144]*KBucket

		mu sync.Mutex
	}
)

type KeyValueStore struct {
	data map[KKey][]byte
	mu   sync.RWMutex
}
