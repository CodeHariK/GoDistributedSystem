package craft

import (
	"net"
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

	server Server

	peerClients map[int64]*Connection

	commitChan chan<- CommitEntry

	ready <-chan any
	quit  chan any
	wg    sync.WaitGroup
	once  sync.Once
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

	// commitChan is the channel where this CM is going to report committed log
	// entries. It's passed in by the client during construction.
	commitChan chan<- CommitEntry

	// newCommitReadyChan is an internal notification channel used by goroutines
	// that commit new entries to the log to notify that these entries may be sent
	// on commitChan.
	newCommitReadyChan chan struct{}

	// Persistent Raft state on all servers
	currentTerm int64
	votedFor    int64
	log         []LogEntry

	// Volatile Raft state on all servers
	commitIndex        int64
	lastApplied        int64
	state              api.CMState
	electionResetEvent time.Time

	// Volatile Raft state on leaders
	nextIndex  map[int64]int64
	matchIndex map[int64]int64
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

type Server struct {
	listener net.Listener
	server   http.Server
}

type CommitEntry struct {
	// Command is the client command being committed.
	Command int64

	// Index is the log index at which the client command is committed.
	Index int64

	// Term is the Raft term at which the client command is committed.
	Term int64
}

type LogEntry struct {
	Command int64
	Term    int64
}
