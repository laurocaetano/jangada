package raft

import (
	"errors"
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

	nextIndexForPeers     map[NodeId]uint64
	nextIndexForPeersLock sync.RWMutex

	peersTerms     map[NodeId]uint64
	peersTermsLock sync.RWMutex

	votedFor     *NodeId
	votedForLock sync.Mutex

	peers           []Node
	transport       Transport
	cancelationChan chan struct{}

	commitIndex     uint64
	commitIndexLock sync.RWMutex

	lastContact     time.Time
	lastContactLock sync.RWMutex
}

func NewRaft(peers []Node, localNode Node, transport Transport) Raft {
	peersTerms := make(map[NodeId]uint64)
	nextIndexForPeers := make(map[NodeId]uint64)
	for _, peer := range peers {
		nextIndexForPeers[peer.id] = 1
	}

	return Raft{
		peers:             peers,
		localNode:         localNode,
		state:             Follower,
		commitIndex:       0,
		currentTerm:       0,
		log:               make(map[uint64]Log),
		transport:         transport,
		peersTerms:        peersTerms,
		nextIndexForPeers: nextIndexForPeers,
	}
}

func (r *Raft) setPeerTerm(peer NodeId, term uint64) {
	r.peersTermsLock.Lock()
	defer r.peersTermsLock.Unlock()

	r.peersTerms[peer] = term
}

func (r *Raft) setNextIndexForPeer(nodeId NodeId, nextLogIndex uint64) {
	r.nextIndexForPeersLock.Lock()
	defer r.nextIndexForPeersLock.Unlock()

	r.nextIndexForPeers[nodeId] = nextLogIndex
}

func (r *Raft) decreaseIndexForPeer(nodeId NodeId) {
	r.nextIndexForPeersLock.Lock()
	defer r.nextIndexForPeersLock.Unlock()

	r.nextIndexForPeers[nodeId]--
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

	if r.log == nil {
		r.log = make(map[uint64]Log)
	}

	for _, entry := range entries {
		r.log[entry.Index] = entry
	}

	// Once this has real IO, it may return actual errors
	return nil
}

func (r *Raft) getLastLog() Log {
	return r.getLogAtIndex(r.getCommitIndex())
}

type appendEntriesPeerResponse struct {
	peer     NodeId
	response AppendEntriesResponse
}

func (r *Raft) sendAppendEntriesRequest(entries []Log) chan appendEntriesPeerResponse {
	responseChan := make(chan appendEntriesPeerResponse, len(r.peers))

	var wg sync.WaitGroup
	wg.Add(len(r.peers))
	go func() {
		wg.Wait()
		close(responseChan)
	}()

	lastCommitedEntry := r.getLastLog()
	for _, peer := range r.peers {
		go func(peer Node) {
			defer wg.Done()

			request := AppendEntriesRequest{
				Term:         r.getCurrentTerm(),
				LeaderNode:   r.leaderNode,
				Entries:      entries,
				PrevLogIndex: lastCommitedEntry.Index,
				PrevLogTerm:  lastCommitedEntry.Term,
			}
			response, error := r.transport.AppendEntries(request, peer)
			if error != nil {
				// Log node is probably offline or unreachable - try again with same log index next time around
			} else {
				responseChan <- appendEntriesPeerResponse{peer.id, response}
			}
		}(peer)
	}

	return responseChan
}

func (r *Raft) sendAppendEntries(newEntries [][]byte) error {
	lastLog := r.getLastLog()

	var newLogEntries []Log
	currentTerm := r.getCurrentTerm()
	nextIndex := lastLog.Index + 1
	for _, entry := range newEntries {
		newLogEntries = append(newLogEntries, Log{
			Term:  currentTerm,
			Index: nextIndex,
			Data:  entry,
		})
		nextIndex++
	}

	responseChan := r.sendAppendEntriesRequest(newLogEntries)

	if len(newLogEntries) > 0 {
		// commit new entries locally
		r.appendEntries(newLogEntries)
	}

	writeQuorum := 1 // Count self
	// simple majority of all nodes (peers + self)
	miniumQuorum := ((len(r.peers) + 1) / 2) + 1

	var replicatedPeers []NodeId
	for res := range responseChan {
		if !res.response.Success {
			r.decreaseIndexForPeer(res.peer)
		} else {
			replicatedPeers = append(replicatedPeers, res.peer)
			writeQuorum++
		}
	}

	// In case of heartbeat, we can skip this check
	if writeQuorum < miniumQuorum && len(newLogEntries) > 0 {
		return errors.New("unable to reach consensus")
	}

	if len(newEntries) > 0 {
		newLastEntry := newLogEntries[len(newEntries)-1]

		r.setCommitIndex(newLastEntry.Index)
		for _, peerId := range replicatedPeers {
			r.setNextIndexForPeer(peerId, newLastEntry.Index+1)
		}
	}

	return nil
}

func (r *Raft) processAppendEntries(appendEntriesRequest AppendEntriesRequest) AppendEntriesResponse {
	if appendEntriesRequest.Term < r.getCurrentTerm() {
		return AppendEntriesResponse{
			Term:    r.getCurrentTerm(),
			Success: false,
		}
	}

	// Set leaderId
	r.setLeader(appendEntriesRequest.LeaderNode)

	// Set state to Follower
	r.setState(Follower)

	lastLogAtIndex := r.getLogAtIndex(appendEntriesRequest.PrevLogIndex)

	if lastLogAtIndex.Term != appendEntriesRequest.PrevLogTerm {
		// Reset timer: set lastContact
		// Still a valid heartbeat
		r.setLastContact()

		return AppendEntriesResponse{
			Term:    r.getCurrentTerm(),
			Success: false,
		}
	}

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
			r.setCommitIndex(lastEntry.Index)
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
