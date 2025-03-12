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
func NewConsensusModule(id int64, peerIds []int64, server *CraftServer, ready <-chan any, commitChan chan<- CommitEntry) *ConsensusModule {
	cm := ConsensusModule{
		id:      id,
		peerIds: peerIds,
		server:  server,

		commitChan:         commitChan,
		newCommitReadyChan: make(chan struct{}, 16),

		state:    api.CMState_FOLLOWER,
		votedFor: -1,

		commitIndex: -1,
		lastApplied: -1,

		nextIndex:  make(map[int64]int64),
		matchIndex: make(map[int64]int64),
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

	go cm.commitChanSender()

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

	lastLogIndex, lastLogTerm := cm.lastLogIndexAndTerm()
	cm.dlog("RequestVote: %+v [currentTerm=%d, votedFor=%d, log index/term=(%d, %d)]", req.Msg, cm.currentTerm, cm.votedFor, lastLogIndex, lastLogTerm)

	if req.Msg.Term > cm.currentTerm {
		cm.dlog("... term out of date in RequestVote")
		cm.becomeFollower(req.Msg.Term)
	}

	reply := api.RequestVoteResponse{
		Term:        cm.currentTerm,
		VoteGranted: false,
	}

	if cm.currentTerm == req.Msg.Term &&
		(cm.votedFor == -1 || cm.votedFor == req.Msg.CandidateId) &&
		(req.Msg.LastLogTerm > lastLogTerm ||
			(req.Msg.LastLogTerm == lastLogTerm && req.Msg.LastLogIndex >= lastLogIndex)) {
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

		// Does our log contain an entry at PrevLogIndex whose term matches
		// PrevLogTerm? Note that in the extreme case of PrevLogIndex=-1 this is
		// vacuously true.
		if req.Msg.PrevLogIndex == -1 ||
			(req.Msg.PrevLogIndex < int64(len(cm.log)) && req.Msg.PrevLogTerm == cm.log[req.Msg.PrevLogIndex].Term) {
			reply.Success = true

			// Find an insertion point - where there's a term mismatch between
			// the existing log starting at PrevLogIndex+1 and the new entries sent
			// in the RPC.
			logInsertIndex := req.Msg.PrevLogIndex + 1
			newEntriesIndex := 0

			for {
				if logInsertIndex >= int64(len(cm.log)) || newEntriesIndex >= len(req.Msg.Entries) {
					break
				}
				if cm.log[logInsertIndex].Term != req.Msg.Entries[newEntriesIndex].Term {
					break
				}
				logInsertIndex++
				newEntriesIndex++
			}
			// At the end of this loop:
			// - logInsertIndex points at the end of the log, or an index where the
			//   term mismatches with an entry from the leader
			// - newEntriesIndex points at the end of Entries, or an index where the
			//   term mismatches with the corresponding log entry
			if newEntriesIndex < len(req.Msg.Entries) {
				cm.dlog("... inserting entries %v from index %d", req.Msg.Entries[newEntriesIndex:], logInsertIndex)
				logentries := req.Msg.Entries[newEntriesIndex:]
				entries := make([]LogEntry, len(logentries))
				for i := range logentries {
					entries[i] = LogEntry{
						Term:    logentries[i].Term,
						Command: logentries[i].Command,
					}
				}
				cm.log = append(cm.log[:logInsertIndex], entries...)
				cm.dlog("... log is now: %v", cm.log)
			}

			// Set commit index.
			if req.Msg.LeaderCommit > cm.commitIndex {
				cm.commitIndex = min(req.Msg.LeaderCommit, int64(len(cm.log)-1))
				cm.dlog("... setting commitIndex=%d", cm.commitIndex)
				cm.newCommitReadyChan <- struct{}{}
			}
		}
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

// Submit submits a new command to the CM. This function doesn't block; clients
// read the commit channel passed in the constructor to be notified of new
// committed entries. It returns true iff this CM is the leader - in which case
// the command is accepted. If false is returned, the client will have to find
// a different CM to submit this command to.
func (cm *ConsensusModule) Submit(command int64) bool {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.dlog("Submit received by %v: %v", cm.state, command)
	if cm.state == api.CMState_LEADER {
		cm.log = append(cm.log, LogEntry{Command: command, Term: cm.currentTerm})
		cm.dlog("... log=%v", cm.log)
		return true
	}
	return false
}

// Stop stops this CM, cleaning up its state. This method returns quickly, but
// it may take a bit of time (up to ~election timeout) for all goroutines to
// exit.
func (cm *ConsensusModule) Stop() {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	cm.state = api.CMState_DEAD
	cm.dlog("becomes Dead")
	close(cm.newCommitReadyChan)
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
			cm.mu.Lock()
			savedLastLogIndex, savedLastLogTerm := cm.lastLogIndexAndTerm()
			cm.mu.Unlock()

			req := api.RequestVoteRequest{
				Term:         savedCurrentTerm,
				CandidateId:  cm.id,
				LastLogIndex: savedLastLogIndex,
				LastLogTerm:  savedLastLogTerm,
			}

			cm.dlog("sending RequestVote to %d: %+v", peerId, &req)

			if peer, exists := cm.server.GetPeerClient(peerId); exists && peer != nil {
				if res, err := peer.client.RequestVote(context.Background(),
					connect.NewRequest(&req)); err == nil {
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

	for _, peerId := range cm.peerIds {
		cm.nextIndex[peerId] = int64(len(cm.log))
		cm.matchIndex[peerId] = -1
	}
	cm.dlog("becomes Leader; term=%d, nextIndex=%v, matchIndex=%v; log=%v", cm.currentTerm, cm.nextIndex, cm.matchIndex, cm.log)

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
			cm.mu.Lock()
			ni := cm.nextIndex[peerId]
			prevLogIndex := ni - 1
			prevLogTerm := int64(-1)
			if prevLogIndex >= 0 {
				prevLogTerm = cm.log[prevLogIndex].Term
			}
			logentries := cm.log[ni:]
			entries := make([]*api.LogEntry, len(logentries))
			for i, le := range logentries {
				entries[i] = &api.LogEntry{
					Term:    le.Term,
					Command: le.Command,
				}
			}

			cm.mu.Unlock()

			req := api.AppendEntriesRequest{
				Term:         savedCurrentTerm,
				LeaderId:     cm.id,
				PrevLogIndex: prevLogIndex,
				PrevLogTerm:  prevLogTerm,
				Entries:      entries,
				LeaderCommit: cm.commitIndex,
			}

			cm.dlog("sending AppendEntries to %v: ni=%d, args=%+v", peerId, ni, &req)

			if peer, exists := cm.server.GetPeerClient(peerId); exists && peer != nil {
				if res, err := peer.client.AppendEntries(
					context.Background(), connect.NewRequest(&req)); err == nil {

					cm.mu.Lock()
					defer cm.mu.Unlock()

					if res.Msg.Term > savedCurrentTerm {
						cm.dlog("term out of date in heartbeat reply")
						cm.becomeFollower(res.Msg.Term)
						return
					}

					if cm.state == api.CMState_LEADER && savedCurrentTerm == res.Msg.Term {
						if res.Msg.Success {
							cm.nextIndex[peerId] = ni + int64(len(entries))
							cm.matchIndex[peerId] = cm.nextIndex[peerId] - 1

							cm.dlog("AppendEntries reply from %d success: nextIndex := %v, matchIndex := %v", peerId, cm.nextIndex, cm.matchIndex)

							savedCommitIndex := cm.commitIndex
							for i := cm.commitIndex + 1; i < int64(len(cm.log)); i++ {
								if cm.log[i].Term == cm.currentTerm {
									matchCount := 1
									for _, peerId := range cm.peerIds {
										if cm.matchIndex[peerId] >= i {
											matchCount++
										}
									}
									if matchCount*2 > len(cm.peerIds)+1 {
										cm.commitIndex = i
									}
								}
							}
							if cm.commitIndex != savedCommitIndex {
								cm.dlog("leader sets commitIndex := %d", cm.commitIndex)
								cm.newCommitReadyChan <- struct{}{}
							}
						} else {
							cm.nextIndex[peerId] = ni - 1
							cm.dlog("AppendEntries reply from %d !success: nextIndex := %d", peerId, ni-1)
						}
					}
				}
			}
		}(peerId)
	}
}

// lastLogIndexAndTerm returns the last log index and the last log entry's term
// (or -1 if there's no log) for this server.
// Expects cm.mu to be locked.
func (cm *ConsensusModule) lastLogIndexAndTerm() (int64, int64) {
	if len(cm.log) > 0 {
		lastIndex := int64(len(cm.log) - 1)
		return lastIndex, cm.log[lastIndex].Term
	} else {
		return -1, -1
	}
}

// commitChanSender is responsible for sending committed entries on
// cm.commitChan. It watches newCommitReadyChan for notifications and calculates
// which new entries are ready to be sent. This method should run in a separate
// background goroutine; cm.commitChan may be buffered and will limit how fast
// the client consumes new committed entries. Returns when newCommitReadyChan is
// closed.
func (cm *ConsensusModule) commitChanSender() {
	for range cm.newCommitReadyChan {
		// Find which entries we have to apply.
		cm.mu.Lock()
		savedTerm := cm.currentTerm
		savedLastApplied := cm.lastApplied
		var entries []LogEntry
		if cm.commitIndex > cm.lastApplied {
			entries = cm.log[cm.lastApplied+1 : cm.commitIndex+1]
			cm.lastApplied = cm.commitIndex
		}
		cm.mu.Unlock()
		cm.dlog("commitChanSender entries=%v, savedLastApplied=%d", entries, savedLastApplied)

		for i := range entries {
			cm.commitChan <- CommitEntry{
				Command: entries[i].Command,
				Index:   savedLastApplied + int64(i) + 1,
				Term:    savedTerm,
			}
		}
	}
	cm.dlog("commitChanSender done")
}
