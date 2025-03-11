package craft

import (
	"context"
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"connectrpc.com/connect"
	"github.com/codeharik/craft/api"
)

// NewConsensusModule creates a new CM with the given ID, list of peer IDs and
// server. The ready channel signals the CM that all peers are connected and
// it's safe to start its state machine.
func NewConsensusModule(id int64, peerIds []int64, server *CraftServer, ready <-chan any) *ConsensusModule {
	cm := ConsensusModule{
		id:       id,
		peerIds:  peerIds,
		server:   server,
		state:    api.CMState_FOLLOWER,
		votedFor: -1,
	}

	go func() {
		// The CM is quiescent until ready is signaled; then, it starts a countdown
		// for leader election.
		<-ready
		cm.mu.Lock()
		cm.electionResetEvent = time.Now()
		cm.mu.Unlock()
		cm.runElectionTimer()
	}()

	return &cm
}

func (cm *ConsensusModule) RequestVote(
	ctx context.Context,
	req *connect.Request[api.RequestVoteRequest],
) (*connect.Response[api.RequestVoteResponse], error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.state == api.CMState_DEAD {
		return nil, connect.NewError(connect.CodeUnavailable,
			errors.New("craft.CraftService.RequestVote CMState_DEAD"))
	}

	cm.dlog("RequestVote: %+v [currentTerm=%d, votedFor=%d]", req.Msg, cm.currentTerm, cm.votedFor)

	if req.Msg.Term > cm.currentTerm {
		cm.dlog("... term out of date in RequestVote")
		cm.becomeFollower(req.Msg.Term)
	}

	reply := api.RequestVoteResponse{
		Term:        cm.currentTerm,
		VoteGranted: false,
	}

	if cm.currentTerm == req.Msg.Term &&
		(cm.votedFor == -1 || cm.votedFor == req.Msg.CandidateId) {
		reply.VoteGranted = true
		cm.votedFor = req.Msg.CandidateId
		cm.electionResetEvent = time.Now()
	}

	cm.dlog("... RequestVote reply: Term:%d Votegranted:%d", reply.Term, reply.VoteGranted)

	return connect.NewResponse(&reply), nil
}

func (cm *ConsensusModule) AppendEntries(
	ctx context.Context,
	req *connect.Request[api.AppendEntriesRequest],
) (*connect.Response[api.AppendEntriesResponse], error) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cm.state == api.CMState_DEAD {
		return nil, connect.NewError(connect.CodeUnavailable,
			errors.New("craft.CraftService.AppendEntries CMState_DEAD"))
	}
	cm.dlog("AppendEntries: %+v", req.Msg)

	if req.Msg.Term > cm.currentTerm {
		cm.dlog("... term out of date in AppendEntries")
		cm.becomeFollower(req.Msg.Term)
	}

	reply := api.AppendEntriesResponse{
		Term:    cm.currentTerm,
		Success: false,
	}

	if req.Msg.Term == cm.currentTerm {
		if cm.state != api.CMState_FOLLOWER {
			cm.becomeFollower(req.Msg.Term)
		}
		cm.electionResetEvent = time.Now()
		reply.Success = true
	}

	cm.dlog("AppendEntries reply: Term:%d Success:%d", reply.Term, reply.Success)

	return connect.NewResponse(&reply), nil
}

// Report reports the state of this CM.
func (cm *ConsensusModule) Report() (id int64, term int64, isLeader bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	return cm.id, cm.currentTerm, cm.state == api.CMState_LEADER
}

// Stop stops this CM, cleaning up its state. This method returns quickly, but
// it may take a bit of time (up to ~election timeout) for all goroutines to
// exit.
func (cm *ConsensusModule) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.state = api.CMState_DEAD
	cm.dlog("becomes Dead")
}

func (cm *ConsensusModule) dlog(format string, args ...any) {
	if DEBUG_CM > 0 {
		format = fmt.Sprintf("[%d] ", cm.id) + format
		log.Printf(format, args...)
	}
}

// electionTimeout generates a pseudo-random election timeout duration.
func (cm *ConsensusModule) electionTimeout() time.Duration {
	// If RAFT_FORCE_MORE_REELECTION is set, stress-test by deliberately
	// generating a hard-coded number very often. This will create collisions
	// between different servers and force more re-elections.
	if len(os.Getenv("RAFT_FORCE_MORE_REELECTION")) > 0 && rand.Intn(3) == 0 {
		return time.Duration(150) * time.Millisecond
	} else {
		return time.Duration(150+rand.Intn(150)) * time.Millisecond
	}
}

// runElectionTimer implements an election timer. It should be launched whenever
// we want to start a timer towards becoming a candidate in a new election.
//
// This function is blocking and should be launched in a separate goroutine;
// it's designed to work for a single (one-shot) election timer, as it exits
// whenever the CM state changes from follower/candidate or the term changes.
func (cm *ConsensusModule) runElectionTimer() {
	timeoutDuration := cm.electionTimeout()
	cm.mu.Lock()
	termStarted := cm.currentTerm
	cm.mu.Unlock()
	cm.dlog("election timer started (%v), term=%d", timeoutDuration, termStarted)

	// This loops until either:
	// - we discover the election timer is no longer needed, or
	// - the election timer expires and this CM becomes a candidate
	// In a follower, this typically keeps running in the background for the
	// duration of the CM's lifetime.
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		<-ticker.C

		cm.mu.Lock()
		if cm.state != api.CMState_CANDIDATE && cm.state != api.CMState_FOLLOWER {
			cm.dlog("in election timer state=%s, bailing out", cm.state)
			cm.mu.Unlock()
			return
		}

		if termStarted != cm.currentTerm {
			cm.dlog("in election timer term changed from %d to %d, bailing out", termStarted, cm.currentTerm)
			cm.mu.Unlock()
			return
		}

		// Start an election if we haven't heard from a leader or haven't voted for
		// someone for the duration of the timeout.
		if elapsed := time.Since(cm.electionResetEvent); elapsed >= timeoutDuration {
			cm.startElection()
			cm.mu.Unlock()
			return
		}
		cm.mu.Unlock()
	}
}

func (s *CraftServer) GetPeerClient(peerId int64) (*Connection, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	// Unlock before making network calls
	// dont blocks other goroutines from accessing cm.server.peerClients
	peer, exists := s.peerClients[peerId]
	return peer, exists
}

// startElection starts a new election with this CM as a candidate.
// Expects cm.mu to be locked.
func (cm *ConsensusModule) startElection() {
	cm.state = api.CMState_CANDIDATE
	cm.currentTerm += 1
	savedCurrentTerm := cm.currentTerm
	cm.electionResetEvent = time.Now()
	cm.votedFor = cm.id
	cm.dlog("becomes Candidate (currentTerm=%d); log=%v", savedCurrentTerm, cm.log)

	votesReceived := 1

	// Send RequestVote RPCs to all other servers concurrently.
	for _, peerId := range cm.peerIds {
		go func(peerId int64) {
			cm.dlog("sending RequestVote to %d: Term:%d CandidateId:%d", peerId, savedCurrentTerm, cm.id)

			if peer, exists := cm.server.GetPeerClient(peerId); exists && peer != nil {
				res, err := peer.client.RequestVote(context.Background(),
					connect.NewRequest(&api.RequestVoteRequest{
						Term:        savedCurrentTerm,
						CandidateId: cm.id,
					}))

				if err == nil {
					cm.mu.Lock()
					defer cm.mu.Unlock()
					cm.dlog("received RequestVoteReply %+v", res)

					if cm.state != api.CMState_CANDIDATE {
						cm.dlog("while waiting for reply, state = %v", cm.state)
						return
					}

					if res.Msg.Term > savedCurrentTerm {
						cm.dlog("term out of date in RequestVoteReply")
						cm.becomeFollower(res.Msg.Term)
						return
					} else if res.Msg.Term == savedCurrentTerm {
						if res.Msg.VoteGranted {
							votesReceived += 1
							if votesReceived*2 > len(cm.peerIds)+1 {
								// Won the election!
								cm.dlog("wins election with %d votes", votesReceived)
								cm.startLeader()
								return
							}
						}
					}
				}
			}
		}(peerId)
	}

	// Run another election timer, in case this election is not successful.
	go cm.runElectionTimer()
}

// becomeFollower makes cm a follower and resets its state.
// Expects cm.mu to be locked.
func (cm *ConsensusModule) becomeFollower(term int64) {
	cm.dlog("becomes Follower with term=%d; log=%v", term, cm.log)
	cm.state = api.CMState_FOLLOWER
	cm.currentTerm = term
	cm.votedFor = -1
	cm.electionResetEvent = time.Now()

	go cm.runElectionTimer()
}

// startLeader switches cm into a leader state and begins process of heartbeats.
// Expects cm.mu to be locked.
func (cm *ConsensusModule) startLeader() {
	cm.state = api.CMState_LEADER
	cm.dlog("becomes Leader; term=%d, log=%v", cm.currentTerm, cm.log)

	go func() {
		ticker := time.NewTicker(50 * time.Millisecond)
		defer ticker.Stop()

		// Send periodic heartbeats, as long as still leader.
		for {
			cm.leaderSendHeartbeats()
			<-ticker.C

			cm.mu.Lock()
			if cm.state != api.CMState_LEADER {
				cm.mu.Unlock()
				return
			}
			cm.mu.Unlock()
		}
	}()
}

// leaderSendHeartbeats sends a round of heartbeats to all peers, collects their
// replies and adjusts cm's state.
func (cm *ConsensusModule) leaderSendHeartbeats() {
	cm.mu.Lock()
	if cm.state != api.CMState_LEADER {
		cm.mu.Unlock()
		return
	}
	savedCurrentTerm := cm.currentTerm
	cm.mu.Unlock()

	for _, peerId := range cm.peerIds {
		go func(peerId int64) {
			cm.dlog("sending AppendEntries to %v: ni=%d, Term=%d LeaderId:%d", peerId, 0, savedCurrentTerm, cm.id)

			if peer, exists := cm.server.GetPeerClient(peerId); exists && peer != nil {
				res, err := peer.client.AppendEntries(context.Background(),
					connect.NewRequest(&api.AppendEntriesRequest{
						Term:     savedCurrentTerm,
						LeaderId: cm.id,
					}))

				if err == nil {
					cm.mu.Lock()
					defer cm.mu.Unlock()
					if res.Msg.Term > savedCurrentTerm {
						cm.dlog("term out of date in heartbeat reply")
						cm.becomeFollower(res.Msg.Term)
						return
					}
				}
			}
		}(peerId)
	}
}
