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

type AppendEntriesRequest struct {
	// Leader's term
	Term uint64 `json:"term"`

	// So that followers can redirect requests to learder
	LeaderNode Node `json:"leader_node"`

	// Index of the log entry immediately preceding new ones
	PrevLogIndex uint64 `json:"prev_log_index"`

	// Term of the previous log entry
	PrevLogTerm uint64 `json:"prev_log_term"`

	// Log entries to store (empty for heartbeat)
	Entries []Log `json:"entries"`

	// Leader's commit index
	LeaderCommit uint64 `json:"prev_log_term"`
}

type AppendEntriesResponse struct {
	// Current term, for leader to update itself
	Term uint64 `json:"term"`

	// true if follower contained entry matching prevLogEntry and prevLogIndex
	Success bool `json:"success"`
}

type Transport interface {
	RequestVote(request RequestVoteRequest, peer Node) (RequestVoteResponse, error)
	AppendEntries(appendEntryRequest AppendEntriesRequest, peer Node) (AppendEntriesResponse, error)
}
