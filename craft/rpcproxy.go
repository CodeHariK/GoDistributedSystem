package craft

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/codeharik/craft/api"
)

func (rpp *RPCProxy) RequestVote(req *api.RequestVoteRequest) (*api.RequestVoteResponse, error) {
	if len(os.Getenv("RAFT_UNRELIABLE_RPC")) > 0 {
		dice := rand.Intn(10)
		if dice == 9 {
			rpp.cm.dlog("drop RequestVote")
			return nil, fmt.Errorf("RPC failed")
		} else if dice == 8 {
			rpp.cm.dlog("delay RequestVote")
			time.Sleep(75 * time.Millisecond)
		}
	} else {
		time.Sleep(time.Duration(1+rand.Intn(5)) * time.Millisecond)
	}

	res, err := rpp.cm.RequestVote(
		context.Background(),
		connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}

func (rpp *RPCProxy) AppendEntries(req *api.AppendEntriesRequest) (*api.AppendEntriesResponse, error) {
	if len(os.Getenv("RAFT_UNRELIABLE_RPC")) > 0 {
		dice := rand.Intn(10)
		if dice == 9 {
			rpp.cm.dlog("drop AppendEntries")
			return nil, fmt.Errorf("RPC failed")
		} else if dice == 8 {
			rpp.cm.dlog("delay AppendEntries")
			time.Sleep(75 * time.Millisecond)
		}
	} else {
		time.Sleep(time.Duration(1+rand.Intn(5)) * time.Millisecond)
	}

	res, err := rpp.cm.AppendEntries(
		context.Background(),
		connect.NewRequest(req))
	if err != nil {
		return nil, err
	}
	return res.Msg, nil
}
