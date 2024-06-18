package raft

import (
	"sync"
	"time"
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
	leaderLock sync.RWMutex

	localNode Node

	state     State
	stateLock sync.RWMutex

	currentTerm     uint64
	currentTermLock sync.RWMutex

	// LogIndex -> Log
	log     map[uint64]Log
	logLock sync.RWMutex

	votedFor     *NodeId
	votedForLock sync.Mutex

	peers           []Node
	transport       Transport
	cancelationChan chan struct{}

	commitIndex     uint64
	commitIndexLock sync.RWMutex

	lastContact     time.Time
	lastContactLock sync.RWMutex

	lastLog     Log
	lastLogLock sync.RWMutex
}

func (r *Raft) setLastContact() {
	r.lastContactLock.Lock()
	defer r.lastContactLock.Unlock()

	r.lastContact = time.Now()
}

func (r *Raft) getLastContact() time.Time {
	r.lastContactLock.RLock()
	defer r.lastContactLock.RUnlock()

	return r.lastContact
}

func (r *Raft) setLeader(leader Node) {
	r.leaderLock.Lock()
	defer r.leaderLock.Unlock()

	r.leaderNode = leader
}

func (r *Raft) getCommitIndex() uint64 {
	r.commitIndexLock.RLock()
	defer r.commitIndexLock.RUnlock()

	return r.commitIndex
}

func (r *Raft) setCommitIndex(newIndex uint64) {
	r.commitIndexLock.Lock()
	defer r.commitIndexLock.Unlock()

	r.commitIndex = newIndex
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

func (r *Raft) getLogAtIndex(index uint64) Log {
	r.logLock.RLock()
	defer r.logLock.RUnlock()

	log, ok := r.log[index]
	if !ok {
		return Log{}
	}

	return log
}

func (r *Raft) deleteLogsFrom(start, end uint64) {
	r.logLock.Lock()
	defer r.logLock.Unlock()

	for i := start; i <= end; i++ {
		delete(r.log, i)
	}
}

func (r *Raft) appendEntries(entries []Log) error {
	r.logLock.Lock()
	defer r.logLock.Unlock()

	for _, entry := range entries {
		r.log[entry.Index] = entry
	}

	// Once this has real IO, it may return actual errors
	return nil
}

// Returns the latest log item, or nothing (empty Log Struct)
func (r *Raft) getLastLog() Log {
	r.lastLogLock.RLock()
	defer r.lastLogLock.RUnlock()

	return r.lastLog
}

func (r *Raft) setLastLog(latest Log) {
	r.lastLogLock.Lock()
	defer r.lastLogLock.Unlock()

	r.lastLog = latest
}

func (r *Raft) processAppendEntries(appendEntriesRequest AppendEntriesRequest) AppendEntriesResponse {
	if appendEntriesRequest.Term < r.getCurrentTerm() {
		return AppendEntriesResponse{
			Term:    r.getCurrentTerm(),
			Success: false,
		}
	}

	lastLogAtIndex := r.getLogAtIndex(appendEntriesRequest.PrevLogIndex)

	if lastLogAtIndex.Term != appendEntriesRequest.PrevLogTerm {
		return AppendEntriesResponse{
			Term:    r.getCurrentTerm(),
			Success: false,
		}
	}

	// Set leaderId
	r.setLeader(appendEntriesRequest.LeaderNode)

	// Set state to Follower
	r.setState(Follower)

	// Handle new entries
	if len(appendEntriesRequest.Entries) > 0 {
		lastLog := r.getLastLog()
		var newEntries []Log

		// Delete conflicting logs
		for i, entry := range appendEntriesRequest.Entries {
			if entry.Index > lastLog.Index {
				newEntries = appendEntriesRequest.Entries[i:]
				break
			}

			logAtIndex := r.getLogAtIndex(entry.Index)

			if entry.Term != logAtIndex.Term {
				// delete log entries since the diff
				r.deleteLogsFrom(entry.Index, lastLog.Index)

				// append everything
				newEntries = appendEntriesRequest.Entries[i:]
				break
			}
		}

		if len(newEntries) > 0 {
			// Append new entries
			r.appendEntries(newEntries)

			lastEntry := newEntries[len(newEntries)-1]
			r.setLastLog(lastEntry)
		}

		lastNewEntry := r.getLastLog()
		if appendEntriesRequest.LeaderCommit > r.getCommitIndex() {
			r.setCommitIndex(min(appendEntriesRequest.LeaderCommit, lastNewEntry.Index))
		}
	}

	// Reset timer: set lastContact
	r.setLastContact()

	return AppendEntriesResponse{
		Term:    r.getCurrentTerm(),
		Success: true,
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
		latestLog := r.getLastLog()
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

	lastLogEntry := r.getLastLog()

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
