package craft

import (
	"net/http"
	"sync"
	"time"

	"github.com/codeharik/craft/api"
	"github.com/codeharik/craft/api/apiconnect"
)

const DEBUG_CM = 1

type CraftServer struct {
	mu sync.Mutex

	serverID int64
	peerIDs  []int64

	cm       *ConsensusModule
	rpcProxy *RPCProxy

	server *http.Server

	peerClients map[int64]*Connection

	ready <-chan any
	quit  chan any
}

type ConsensusModule struct {
	// mu protects concurrent access to a CM.
	mu sync.Mutex

	// id is the server id of this CM.
	id int64

	// peerIds lists the IDs of our peers in the cluster.
	peerIds []int64

	// server is the server containing this CM. It's used to issue RPC calls
	// to peers.
	server *CraftServer

	// Persistent Raft state on all servers
	currentTerm int64
	votedFor    int64
	log         []api.LogEntry

	// Volatile Raft state on all servers
	state              api.CMState
	electionResetEvent time.Time
}

// RPCProxy is a trivial pass-thru proxy type for ConsensusModule's RPC methods.
// It's useful for:
//   - Simulating a small delay in RPC transmission.
//   - Avoiding running into https://github.com/golang/go/issues/19957
//   - Simulating possible unreliable connections by delaying some messages
//     significantly and dropping others when RAFT_UNRELIABLE_RPC is set.
type RPCProxy struct {
	cm *ConsensusModule
}

type Connection struct {
	client apiconnect.CraftServiceClient
	http   *http.Client
}
