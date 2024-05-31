package raft

type RequestVoteRequest struct {
	Term         uint64
	CandidateId  ServerId
	LastLogIndex uint64
	LastLogTerm  uint64
}

type RequestVoteResponse struct {
	Term        uint64
	VoteGranted bool
}

type RequestVoteRpc interface {
	RequestVote(request RequestVoteRequest) (RequestVoteResponse, error)
}

type TcpClient struct{}
