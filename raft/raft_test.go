package raft

import (
	"testing"
)

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

func (f FakeTestTransport) RequestVote(request RequestVoteRequest, peer Node) (RequestVoteResponse, error) {
	return f.responseForNode[peer], nil
}
