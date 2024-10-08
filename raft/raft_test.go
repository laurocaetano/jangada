package raft

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

// The tests in this function are purposely verbose.
// They are written with lots of repetition to make it clear
// what are the pre and post condition for each case.
//
// The aim in these test files is to give an overview, top to bottom, of what it is
// testing, without the need to jump to helper functions and etc.
//
// Perhaps when the code evolves, I will re-factor the test cases to make them prettier
// and less verbose.
func TestHandleVote(t *testing.T) {
	candidateNode := Node{
		id:      "peer-one",
		address: "10.0.0.1",
	}

	localNode := Node{
		id:      "peer-three",
		address: "10.0.0.3",
	}

	lastLogEntry := Log{
		Index: 20,
		Term:  10,
	}

	raft := Raft{
		localNode:   localNode,
		state:       Follower,
		peers:       []Node{},
		currentTerm: 10,
	}

	raft.appendEntries([]Log{lastLogEntry})
	raft.setCommitIndex(lastLogEntry.Index)

	// In case the candidate's term is lower
	voteRequest := RequestVoteRequest{
		Term:         raft.getCurrentTerm() - 1,
		CandidateId:  candidateNode.id,
		LastLogIndex: 19,
		LastLogTerm:  9,
	}

	// The vote should not be granted
	response := raft.handleVoteRequest(voteRequest)
	if response.VoteGranted {
		t.Errorf("expected vote not to be granted for request %v", voteRequest)
	}

	// In case vote has been already granted
	var votedFor NodeId = "123"
	raft.votedFor = &votedFor
	voteRequest = RequestVoteRequest{
		Term:         raft.getCurrentTerm() + 1,
		CandidateId:  candidateNode.id,
		LastLogIndex: 19,
		LastLogTerm:  9,
	}

	// The vote should not be granted
	response = raft.handleVoteRequest(voteRequest)
	if response.VoteGranted {
		t.Errorf("expected vote not to be granted for request %v", voteRequest)
	}

	// Clear last vote
	raft.votedFor = nil

	// In case there is no vote yet but candidate's log index is lower
	voteRequest = RequestVoteRequest{
		Term:         raft.getCurrentTerm() + 1,
		CandidateId:  candidateNode.id,
		LastLogIndex: lastLogEntry.Index - 1,
		LastLogTerm:  9,
	}

	// The vote should not be granted
	response = raft.handleVoteRequest(voteRequest)
	if response.VoteGranted {
		t.Errorf("expected vote not to be granted for request %v", voteRequest)
	}

	// In case there is no vote yet but candidate's log term is lower
	voteRequest = RequestVoteRequest{
		Term:         raft.getCurrentTerm() + 1,
		CandidateId:  candidateNode.id,
		LastLogIndex: lastLogEntry.Index,
		LastLogTerm:  lastLogEntry.Term - 1,
	}

	// The vote should not be granted
	response = raft.handleVoteRequest(voteRequest)
	if response.VoteGranted {
		t.Errorf("expected vote not to be granted for request %v", voteRequest)
	}

	// In case there is no vote and the candidates' term is at least as
	// up-to-date
	voteRequest = RequestVoteRequest{
		Term:         raft.getCurrentTerm() + 1,
		CandidateId:  candidateNode.id,
		LastLogIndex: lastLogEntry.Index,
		LastLogTerm:  lastLogEntry.Term,
	}

	// The vote should be granted
	response = raft.handleVoteRequest(voteRequest)
	if !response.VoteGranted && raft.votedFor != &candidateNode.id {
		t.Errorf("expected vote to be granted for request %v", voteRequest)
	}

	// In case the last vote was for the same candidate
	raft.votedFor = &candidateNode.id
	voteRequest = RequestVoteRequest{
		Term:         raft.getCurrentTerm() + 1,
		CandidateId:  candidateNode.id,
		LastLogIndex: lastLogEntry.Index,
		LastLogTerm:  lastLogEntry.Term,
	}

	// The vote should be granted
	response = raft.handleVoteRequest(voteRequest)
	if !response.VoteGranted && raft.votedFor != &candidateNode.id {
		t.Errorf("expected vote to be granted for request %v", voteRequest)
	}
}

func TestLeaderElectionSuccessful(t *testing.T) {
	peerOne := Node{
		id:      "peer-one",
		address: "10.0.0.1",
	}

	peerTwo := Node{
		id:      "peer-two",
		address: "10.0.0.2",
	}

	localNode := Node{
		id:      "peer-three",
		address: "10.0.0.3",
	}

	fakeResponse := make(map[Node]RequestVoteResponse)
	fakeResponse[peerOne] = RequestVoteResponse{
		Term:        0,
		VoteGranted: true,
	}

	fakeResponse[peerTwo] = RequestVoteResponse{
		Term:        0,
		VoteGranted: true,
	}

	fakeTransport := FakeTestTransport{fakeResponse, nil}

	raft := Raft{
		localNode: localNode,
		state:     Follower,
		peers:     []Node{peerOne, peerTwo},
		transport: fakeTransport,
	}

	raft.startElection()

	// Test the case when all candidates do not grant vote
	if raft.getCurrentState() != Leader {
		t.Errorf("expected successful election to lead node in Leader state, but got: %v", raft.getCurrentState())
	}
}

func TestLeaderElectionNotSuccessful(t *testing.T) {
	peerOne := Node{
		id:      "peer-one",
		address: "10.0.0.1",
	}

	peerTwo := Node{
		id:      "peer-two",
		address: "10.0.0.2",
	}

	localNode := Node{
		id:      "peer-three",
		address: "10.0.0.3",
	}

	fakeResponse := make(map[Node]RequestVoteResponse)
	fakeResponse[peerOne] = RequestVoteResponse{
		Term:        0,
		VoteGranted: false,
	}

	fakeResponse[peerTwo] = RequestVoteResponse{
		Term:        0,
		VoteGranted: false,
	}

	fakeTransport := FakeTestTransport{fakeResponse, nil}

	raft := Raft{
		localNode: localNode,
		state:     Follower,
		peers:     []Node{peerOne, peerTwo},
		transport: fakeTransport,
	}

	raft.startElection()
}

func TestSendAppendEntriesIncreasesPeerNextIndex(t *testing.T) {
	// A raft with no log entries yet
	peer1 := Node{
		id:      "node01",
		address: "0.0.0.1",
	}

	peer2 := Node{
		id:      "node02",
		address: "0.0.0.2",
	}

	leaderNode := Node{
		id:      "leaderNode",
		address: "0.0.0.3",
	}

	peers := []Node{peer1, peer2}

	fakeResponseMap := make(map[Node]AppendEntriesResponse)

	fakeResponseMap[peer1] = AppendEntriesResponse{
		Term:    1,
		Success: true,
	}

	fakeResponseMap[peer2] = AppendEntriesResponse{
		Term:    1,
		Success: false,
	}

	fakeTransport := FakeTestTransport{nil, fakeResponseMap}

	raft := NewRaft(peers, leaderNode, fakeTransport)
	// Fake this current instance is the leader
	raft.setState(Leader)
	raft.setCurrentTerm(1)

	newEntry := []byte("my new entry")
	expectedCommitedIndex := raft.getCommitIndex() + 1

	response := raft.sendAppendEntries([][]byte{newEntry})

	if response != nil {
		t.Errorf("it should have been successful, but it was not: %v", response)
	}

	// It should move the commit index
	if raft.getCommitIndex() != expectedCommitedIndex {
		t.Errorf("expected commit index to be %v, but was %v", expectedCommitedIndex, raft.getCommitIndex())
	}

	// it should only increase index for commited peers
	expectedPeer1Index := raft.getCommitIndex() + 1
	if raft.nextIndexForPeers[peer1.id] != expectedPeer1Index {
		t.Errorf("nextIndexFor peer %v expected to be %v, got: %v",
			peer1.id,
			expectedPeer1Index,
			raft.nextIndexForPeers[peer1.id],
		)
	}

	// it should only increase index for commited peers
	expectedPeer2Index := raft.getCommitIndex() - 1
	if raft.nextIndexForPeers[peer2.id] != expectedPeer2Index {
		t.Errorf("nextIndexFor peer %v expected to be %v, got: %v",
			peer2.id,
			expectedPeer2Index,
			raft.nextIndexForPeers[peer2.id],
		)
	}
}

func TestSendAppendEntriesWithoutEntries(t *testing.T) {
	// A raft with no log entries yet
	peer1 := Node{
		id:      "node01",
		address: "0.0.0.1",
	}

	peer2 := Node{
		id:      "node02",
		address: "0.0.0.2",
	}

	leaderNode := Node{
		id:      "leaderNode",
		address: "0.0.0.3",
	}

	peers := []Node{peer1, peer2}

	fakeResponseMap := make(map[Node]AppendEntriesResponse)

	fakeResponseMap[peer1] = AppendEntriesResponse{
		Term:    1,
		Success: true,
	}

	fakeResponseMap[peer2] = AppendEntriesResponse{
		Term:    1,
		Success: true,
	}

	fakeTransport := FakeTestTransport{nil, fakeResponseMap}

	raft := NewRaft(peers, leaderNode, fakeTransport)
	// Fake this current instance is the leader
	raft.setState(Leader)
	raft.setCurrentTerm(1)

	response := raft.sendAppendEntries([][]byte{})

	if response != nil {
		t.Errorf("it should have been successful, but it was not: %v", response)
	}

	// It should not move the commit index for heartbeat requests
	if raft.getCommitIndex() > 0 {
		t.Errorf("did not expect commit index to have increased in case of heartbeat")
	}

	// it should not increase indexes on heartbeat requests
	expectedPeerIndex := raft.getCommitIndex() + 1
	for _, peer := range peers {
		if raft.nextIndexForPeers[peer.id] != expectedPeerIndex {
			t.Errorf("nextIndexFor peer %v expected to be %v, got: %v",
				peer.id,
				expectedPeerIndex,
				raft.nextIndexForPeers[peer.id],
			)
		}
	}
}

func TestSendAppendEntriesWithoutQuorum(t *testing.T) {
	// A raft with no log entries yet
	peer1 := Node{
		id:      "node01",
		address: "0.0.0.1",
	}

	peer2 := Node{
		id:      "node02",
		address: "0.0.0.2",
	}

	leaderNode := Node{
		id:      "leaderNode",
		address: "0.0.0.3",
	}

	peers := []Node{peer1, peer2}

	fakeResponseMap := make(map[Node]AppendEntriesResponse)

	fakeResponseMap[peer1] = AppendEntriesResponse{
		Term:    1,
		Success: false,
	}

	fakeResponseMap[peer2] = AppendEntriesResponse{
		Term:    1,
		Success: false,
	}

	fakeTransport := FakeTestTransport{nil, fakeResponseMap}

	raft := NewRaft(peers, leaderNode, fakeTransport)
	// Fake this current instance is the leader
	raft.setState(Leader)
	raft.setCurrentTerm(1)

	newEntry := []byte("My new Entry")

	response := raft.sendAppendEntries([][]byte{newEntry})

	if response.Error() != errors.New("unable to reach consensus").Error() {
		t.Errorf("it should have not reached consensus, but it did")
	}

	// in case the peers did not accept the append entries, do not update latest commit index
	if raft.getCommitIndex() > 0 {
		t.Errorf("did not expect commit index to increase when there is no consensus")
	}

	// it should have decreased the nextIndexForPeers
	for _, peer := range peers {
		if raft.nextIndexForPeers[peer.id] > 0 {
			t.Errorf("nextIndexFor peer %v expected to be %v, got: %v",
				peer.id,
				0,
				raft.nextIndexForPeers[peer.id],
			)
		}
	}
}

func TestProcessAppendEntries(t *testing.T) {
	logMap := make(map[uint64]Log)
	log := Log{
		Index: 99,
		Term:  99,
	}
	logMap[log.Index] = log

	raft := Raft{
		currentTerm: 11,
		log:         logMap,
	}

	appendEntry := AppendEntriesRequest{
		Term: raft.getCurrentTerm() - 1,
	}

	// In case the given term is lower, return false
	response := raft.processAppendEntries(appendEntry)
	if response.Success {
		t.Errorf("expected append entry to return false")
	}

	appendEntry = AppendEntriesRequest{
		Term:        raft.getCurrentTerm(),
		PrevLogTerm: log.Term + 1,
	}

	// In case the previous logIndex has a diff term than the request, return false
	response = raft.processAppendEntries(appendEntry)
	if response.Success {
		t.Errorf("expected append entry to return false")
	}

	leaderNode := Node{
		id:      "123",
		address: "0.0.0.0",
	}

	appendEntry = AppendEntriesRequest{
		Term:         raft.getCurrentTerm(),
		PrevLogTerm:  log.Term,
		PrevLogIndex: log.Index,
		LeaderNode:   leaderNode,
	}

	// In case it's a valid request with no log entry, treat it as a heartbeat
	response = raft.processAppendEntries(appendEntry)
	if !response.Success {
		t.Errorf("expected append entry to return true ")
	}
	if raft.leaderNode != leaderNode {
		t.Errorf("expected leader to be %v, but was %v", leaderNode, raft.leaderNode)
	}
	if raft.getCurrentState() != Follower {
		t.Errorf("expected state to be Follower, but was %v", raft.getCurrentState())
	}

	newLogEntry := Log{
		Term:  log.Term + 1,
		Index: log.Index + 1,
	}
	appendEntry = AppendEntriesRequest{
		Term:         raft.getCurrentTerm(),
		PrevLogTerm:  log.Term,
		PrevLogIndex: log.Index,
		LeaderNode:   leaderNode,
		Entries:      []Log{newLogEntry},
	}

	// In case it's a valid request with only a new entry
	response = raft.processAppendEntries(appendEntry)
	if !response.Success {
		t.Errorf("expected append entry to return true ")
	}
	if !reflect.DeepEqual(raft.getLastLog(), newLogEntry) {
		t.Errorf("expected the last log to be set to %v, but was %v", newLogEntry, raft.getLastLog())
	}

	appendEntry = AppendEntriesRequest{
		Term:         raft.getCurrentTerm(),
		PrevLogTerm:  log.Term,
		PrevLogIndex: log.Index,
		LeaderNode:   leaderNode,
		// In case of entry pointing to an existing entry
		Entries: []Log{log},
	}

	// The last entry should still apply
	response = raft.processAppendEntries(appendEntry)
	if !response.Success {
		t.Errorf("expected append entry to return true ")
	}
	if !reflect.DeepEqual(raft.getLastLog(), newLogEntry) {
		t.Errorf("expected the last log to be set to %v, but was %v", newLogEntry, raft.getLastLog())
	}

	updatedEntry := Log{
		Term:  log.Term + 1,
		Index: log.Index,
	}
	appendEntry = AppendEntriesRequest{
		Term:         raft.getCurrentTerm(),
		PrevLogTerm:  log.Term,
		PrevLogIndex: log.Index,
		LeaderNode:   leaderNode,
		Entries:      []Log{updatedEntry},
		LeaderCommit: log.Term + 1,
	}

	startTime := time.Now()
	// It should delete the previous entry and update with the new one
	response = raft.processAppendEntries(appendEntry)
	if !reflect.DeepEqual(raft.getLastLog(), updatedEntry) {
		t.Errorf("expected the last log to be set to %v, but was %v", updatedEntry, raft.getLastLog())
	}
	if raft.getCommitIndex() != updatedEntry.Index {
		t.Errorf("expected leader commit to be %v, but was %v", updatedEntry.Index, raft.getCommitIndex())
	}

	// it shoud set the last contact
	if !startTime.Before(raft.getLastContact()) {
		t.Errorf("expected the last contact to be after %v, but it was set to %v", startTime, raft.getLastContact())
	}
}

type FakeTestTransport struct {
	responseForNode            map[Node]RequestVoteResponse
	voteRequestResponseForNode map[Node]AppendEntriesResponse
}

func (f FakeTestTransport) AppendEntries(AppendEntriesRequest AppendEntriesRequest, peer Node) (AppendEntriesResponse, error) {
	return f.voteRequestResponseForNode[peer], nil
}

func (f FakeTestTransport) RequestVote(request RequestVoteRequest, peer Node) (RequestVoteResponse, error) {
	return f.responseForNode[peer], nil
}
