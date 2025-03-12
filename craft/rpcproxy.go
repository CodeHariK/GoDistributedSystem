package craft

import (
	"sync"
)

// Proxy provides access to the RPC proxy this server is using; this is only
// for testing purposes to simulate faults.
func (s *CraftServer) Proxy() *RPCProxy {
	return s.rpcProxy
}

// RPCProxy is a pass-thru proxy server for ConsensusModule's RPC methods. It
// serves RPC requests made to a CM and manipulates them before forwarding to
// the CM itself.
//
// It's useful for things like:
//   - Simulating dropping of RPC calls
//   - Simulating a small delay in RPC transmission.
//   - Simulating possible unreliable connections by delaying some messages
//     significantly and dropping others when RAFT_UNRELIABLE_RPC is set.
type RPCProxy struct {
	mu sync.Mutex
	cm *ConsensusModule

	// numCallsBeforeDrop is used to control dropping RPC calls:
	//   -1: means we're not dropping any calls
	//    0: means we're dropping all calls now
	//   >0: means we'll start dropping calls after this number is made
	numCallsBeforeDrop int64
}

func NewProxy(cm *ConsensusModule) *RPCProxy {
	return &RPCProxy{
		cm:                 cm,
		numCallsBeforeDrop: -1,
	}
}

// func (rpp *RPCProxy) RequestVote(req *api.RequestVoteRequest) (*api.RequestVoteResponse, error) {
// 	if len(os.Getenv("RAFT_UNRELIABLE_RPC")) > 0 {
// 		dice := rand.Intn(10)
// 		if dice == 9 {
// 			rpp.cm.dlog("drop RequestVote")
// 			return nil, fmt.Errorf("RPC failed")
// 		} else if dice == 8 {
// 			rpp.cm.dlog("delay RequestVote")
// 			time.Sleep(75 * time.Millisecond)
// 		}
// 	} else {
// 		time.Sleep(time.Duration(1+rand.Intn(5)) * time.Millisecond)
// 	}

// 	res, err := rpp.cm.RequestVote(
// 		context.Background(),
// 		connect.NewRequest(req))
// 	if err != nil {
// 		return nil, err
// 	}
// 	return res.Msg, nil
// }

// func (rpp *RPCProxy) AppendEntries(req *api.AppendEntriesRequest) (*api.AppendEntriesResponse, error) {
// 	if len(os.Getenv("RAFT_UNRELIABLE_RPC")) > 0 {
// 		dice := rand.Intn(10)
// 		if dice == 9 {
// 			rpp.cm.dlog("drop AppendEntries")
// 			return nil, fmt.Errorf("RPC failed")
// 		} else if dice == 8 {
// 			rpp.cm.dlog("delay AppendEntries")
// 			time.Sleep(75 * time.Millisecond)
// 		}
// 	} else {
// 		time.Sleep(time.Duration(1+rand.Intn(5)) * time.Millisecond)
// 	}

// 	res, err := rpp.cm.AppendEntries(
// 		context.Background(),
// 		connect.NewRequest(req))
// 	if err != nil {
// 		return nil, err
// 	}
// 	return res.Msg, nil
// }

// DropCallsAfterN instruct the proxy to drop calls after n are made from this
// point.
func (rpp *RPCProxy) DropCallsAfterN(n int64) {
	rpp.mu.Lock()
	defer rpp.mu.Unlock()

	rpp.numCallsBeforeDrop = n
}

func (rpp *RPCProxy) DontDropCalls() {
	rpp.mu.Lock()
	defer rpp.mu.Unlock()

	rpp.numCallsBeforeDrop = -1
}
