package raft

type RequestVoteRequest struct {
	Term         uint64 `json:"term"`
	CandidateId  NodeId `json:"candidate_id"`
	LastLogIndex uint64 `json:"last_log_index"`
	LastLogTerm  uint64 `json:"last_log_term"`
}

type RequestVoteResponse struct {
	Term        uint64 `json:"term"`
	VoteGranted bool   `json:"vote_granted"`
}

type Transport interface {
	RequestVote(request RequestVoteRequest, peer Node) (RequestVoteResponse, error)
}
