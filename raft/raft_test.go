package raft

import (
	"testing"
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
		log:         []Log{lastLogEntry},
	}

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

	fakeTransport := FakeTestTransport{fakeResponse}

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

	fakeTransport := FakeTestTransport{fakeResponse}

	raft := Raft{
		localNode: localNode,
		state:     Follower,
		peers:     []Node{peerOne, peerTwo},
		transport: fakeTransport,
	}

	raft.startElection()
}

type FakeTestTransport struct {
	responseForNode map[Node]RequestVoteResponse
}

func (f FakeTestTransport) AppendEntries(AppendEntriesRequest AppendEntriesRequest) (AppendEntriesResponse, error) {
	return AppendEntriesResponse{}, nil
}

func (f FakeTestTransport) RequestVote(request RequestVoteRequest, peer Node) (RequestVoteResponse, error) {
	return f.responseForNode[peer], nil
}
