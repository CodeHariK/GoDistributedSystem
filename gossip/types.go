package gossip

import (
	"sync"

	"github.com/codeharik/gossip/api/apiconnect"
)

// BROADCAST_COUNT = 10, Nodes = 10^7
// 10^0  -> 10^1  -> 10^2  -> 10^3  -> ... -> 10^(n-1) -> 10^n
//
// Probability
// 1     -> 10^-6 -> 10^-5 -> 10^-4 -> ... ->          -> 10
//
// n = 10 | idlemax = 10 ^ 10 = 10^10

//
// BROADCAST_NETWORK
//
// NODE_NETWORK
//
// send message from node to BROADCAST_NETWORK, it converges fast, it assigns MessageID or NodeID,
// if ID available, it broadcasts to all peers else sends error
//

const BOOTSTRAP = uint64(1 << 63)

type GossipNode struct {
	ID        uint64
	Addr      string
	StartTime int64

	APPROX_NUM_NODE uint64

	MIN_PEER        int // min number of peers remembered, otherwise get more peers
	MAX_PEER        int // max number of peers remembered, otherwise remove excess peers
	BROADCAST_COUNT int // how many peers message to be forwarded to

	Bootstraps     map[uint64]GossipNodeInfo
	BootstrapsLock sync.Mutex

	Peers     map[uint64]GossipNodeInfo
	PeersLock sync.Mutex
}

type GossipNodeInfo struct {
	ID         uint64
	Addr       string
	StartTime  int64
	connection apiconnect.GossipServiceClient
}
