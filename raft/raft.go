package raft

import (
	"sync"
)

type (
	State  uint
	NodeId string
)

const (
	Follower State = iota
	Candidate
	Leader
)

type Log struct {
	Index uint64
	Term  uint64
	Data  []byte
}

type Node struct {
	id      NodeId
	address string
}

type Raft struct {
	leaderNode Node
	localNode  Node

	state     State
	stateLock sync.RWMutex

	currentTerm     uint64
	currentTermLock sync.RWMutex

	log     []Log
	logLock sync.RWMutex

	votedFor     *NodeId
	votedForLock sync.Mutex

	peers           []Node
	transport       Transport
	cancelationChan chan struct{}
}

func (r *Raft) getCurrentState() State {
	r.stateLock.RLock()
	defer r.stateLock.RUnlock()
	return r.state
}

func (r *Raft) setState(state State) {
	r.stateLock.Lock()
	defer r.stateLock.Unlock()

	r.state = state
}

func (r *Raft) setCurrentTerm(newTerm uint64) {
	r.currentTermLock.Lock()
	defer r.currentTermLock.Unlock()
	r.currentTerm = newTerm
}

func (r *Raft) getCurrentTerm() uint64 {
	r.currentTermLock.RLock()
	defer r.currentTermLock.RUnlock()
	return r.currentTerm
}

// Returns the latest log item, or nothing (empty Log Struct)
func (r *Raft) getLatestLog() Log {
	r.logLock.RLock()
	defer r.logLock.RUnlock()

	if len(r.log) <= 0 {
		return Log{}
	} else {
		return r.log[len(r.log)-1]
	}
}

func (r *Raft) handleVoteRequest(voteRequest RequestVoteRequest) RequestVoteResponse {
	r.votedForLock.Lock()
	defer r.votedForLock.Unlock()

	if voteRequest.Term < r.getCurrentTerm() {
		return RequestVoteResponse{
			Term:        r.getCurrentTerm(),
			VoteGranted: false,
		}
	}

	// if has not voted yet for the current term, or has voted for the same candidateId
	if r.votedFor == nil || *r.votedFor == voteRequest.CandidateId {
		latestLog := r.getLatestLog()
		// Only grant vote in case candidate's log is at least as up-to-date as us
		if voteRequest.LastLogTerm >= latestLog.Term && voteRequest.LastLogIndex >= latestLog.Index {
			// Save the vote for the current term
			r.votedFor = &voteRequest.CandidateId

			return RequestVoteResponse{
				Term:        r.getCurrentTerm(),
				VoteGranted: true,
			}
		}
	}

	return RequestVoteResponse{
		Term:        r.getCurrentTerm(),
		VoteGranted: false,
	}
}

func (r *Raft) startElection() {
	// Change state to Candidate
	r.setState(Candidate)

	newTerm := r.getCurrentTerm() + 1
	r.setCurrentTerm(newTerm)

	// Pending: reset timeout

	// Candidate always starts with a vote to itself
	grantedVotes := 1

	lastLogEntry := r.getLatestLog()

	// send request votes
	voteResponseChan := make(chan RequestVoteResponse, len(r.peers))

	// Local is not part of peers
	// Quorum is a simple majority
	// This is concurrent safe as adding/removing peers is yet not supported
	votesNeeded := ((len(r.peers) + 1) / 2) + 1

	voteRequest := RequestVoteRequest{
		Term:         newTerm,
		CandidateId:  r.localNode.id,
		LastLogIndex: lastLogEntry.Index,
		LastLogTerm:  lastLogEntry.Term,
	}

	for _, peer := range r.peers {
		go func(electionChannel chan<- RequestVoteResponse, peer Node) {
			response, err := r.transport.RequestVote(voteRequest, peer)

			if err == nil {
				electionChannel <- response
			}
		}(voteResponseChan, peer)
	}

	select {
	case voteResponse := <-voteResponseChan:
		if voteResponse.VoteGranted {
			grantedVotes++
		}

		if grantedVotes >= votesNeeded {
			r.setState(Leader)
			return
		}

	case _ = <-r.cancelationChan:
		return
	}
}
