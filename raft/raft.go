package raft

type Log struct {
	Index uint64
	Term  uint64
	Data  []byte
}

type ServerId string

type Server struct {
	Id          ServerId
	Address     string
	CurrentTerm uint64
	VotedFor    ServerId
	Log         []Log
}
