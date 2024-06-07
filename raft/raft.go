package raft

type (
	State    uint
	ServerId string
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

type Server struct {
	id      ServerId
	address string
}

type RaftState struct {
	currentTerm uint64
	log         []Log
	peers       []Server
}
