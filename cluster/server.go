package cluster

import (
	"log"
	"time"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// Server is a cloud-cluster server which handles a serf, raft and a consul agent instances
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

// NewServer creates a new cloud-cluster server
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

// IsLeader returns whether or not this server is leader in a raft cluster
func (s *Server) IsLeader() bool {
	return s.Raft.State() == raft.Leader
}

// Apply is shorthand for (*Server).Raft.Apply(cmd []byte, timeout time.Duration) raft.ApplyFuture
func (s *Server) Apply(cmd []byte, timeout time.Duration) raft.ApplyFuture {
	return s.Raft.Apply(cmd, timeout)
}
