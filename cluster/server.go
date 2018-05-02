package cluster

import (
	"log"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

type Server struct {
	logger *log.Logger
	Config *Config

	// Serf
	Serf         *serf.Serf
	localEventCh chan serf.Event
	EventCh      chan serf.Event

	// Raft
	Raft     *raft.Raft
	LeaderCh chan bool

	// Consul
	catalog *consul.Catalog
	Agent   *consul.Agent

	reconcileCh chan serf.Member
}

func NewServer(config *Config, logger *log.Logger) *Server {
	s := &Server{
		logger:       logger,
		Config:       config,
		reconcileCh:  make(chan serf.Member, 10),
		localEventCh: make(chan serf.Event, 10),
		EventCh:      make(chan serf.Event, 10),
		LeaderCh:     make(chan bool, 10),
	}

	return s
}

func (s *Server) IsLeader() bool {
	return s.Raft.State() == raft.Leader
}

func (s *Server) Apply(cmd []byte, timeout time.Duration) raft.ApplyFuture {
	return s.Raft.Apply(cmd, timeout)
}
