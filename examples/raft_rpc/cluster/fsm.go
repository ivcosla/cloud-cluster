package cluster

import (
	"encoding/json"
	"io"

	"github.com/hashicorp/raft"
)

// --- state

type State struct {
	keys map[int]int
}

func (s *State) Set(key, value int) {
	s.keys[key] = value
}

type SetRequest struct {
	Key, Value int
}

func NewRequest(key, value int) []byte {
	req := SetRequest{
		Key:   key,
		Value: value,
	}

	data, err := json.Marshal(req)
	if err != nil {
		panic(err)
	}

	return data
}

// --- fsm

type SimpleFSM struct {
	s *State
}

func NewFSM() raft.FSM {
	return &SimpleFSM{
		s: &State{
			keys: map[int]int{},
		},
	}
}

func (f *SimpleFSM) State() *State {
	return f.s
}

func (f *SimpleFSM) Apply(l *raft.Log) interface{} {
	data := l.Data

	var request SetRequest
	if err := json.Unmarshal(data, &request); err != nil {
		panic(err)
	}

	f.s.Set(request.Key, request.Value)

	return nil
}

func (f *SimpleFSM) Snapshot() (raft.FSMSnapshot, error) {
	return nil, nil
}

func (f *SimpleFSM) Restore(io.ReadCloser) error {
	return nil
}
