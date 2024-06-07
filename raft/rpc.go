package raft

type RequestVoteRequest struct {
	Term         uint64   `json:"term,omitempty"`
	CandidateId  ServerId `json:"candidate_id,omitempty"`
	LastLogIndex uint64   `json:"last_log_index,omitempty"`
	LastLogTerm  uint64   `json:"last_log_term,omitempty"`
}

type RequestVoteResponse struct {
	Term        uint64 `json:"term,omitempty"`
	VoteGranted bool   `json:"vote_granted,omitempty"`
}

type RequestVoteRpc interface {
	RequestVote(request RequestVoteRequest) (RequestVoteResponse, error)
}
