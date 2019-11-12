package cluster

import (
	"strings"
	"sync/atomic"
	"time"

	"github.com/hashicorp/go-discover"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// DiscoverBootstrapLimit is the limit of attempts for the serf agent to try to join a cluster
var DiscoverBootstrapLimit = 10

// SetupSerf starts serf in a gouroutine and handles it's events
func (s *Server) SetupSerf() error {
	conf := s.Config.SerfConfig
	conf.Init()

	conf.Tags["id"] = s.Config.NodeName

	for k, v := range s.Config.Tags {
		conf.Tags[k] = v
	}

	conf.NodeName = s.Config.NodeName
	conf.MemberlistConfig.LogOutput = s.Config.LogOutput
	conf.LogOutput = s.Config.LogOutput
	conf.EventCh = s.localEventCh

	client, err := serf.Create(conf)
	if err != nil {
		return err
	}

	s.Serf = client
	go s.eventHandler()

	// start retryJoin discover
	go s.setupRetryJoin()

	return nil
}

func (s *Server) eventHandler() {
	for e := range s.localEventCh {

		s.EventCh <- e

		switch e.EventType() {
		case serf.EventMemberJoin:
			s.nodeJoin(e.(serf.MemberEvent))
			s.localMemberEvent(e.(serf.MemberEvent))
		case serf.EventMemberLeave, serf.EventMemberFailed:
			s.nodeLeave(e.(serf.MemberEvent))
			s.localMemberEvent(e.(serf.MemberEvent))
		}
	}
}

func (s *Server) localMemberEvent(me serf.MemberEvent) {
	if s.Raft == nil {
		return
	}

	if !s.IsLeader() {
		return
	}

	for _, m := range me.Members {
		select {
		case s.reconcileCh <- m:
		default:
		}
	}
}

func (s *Server) nodeJoin(me serf.MemberEvent) {
	for _, m := range me.Members {
		s.logger.Printf("[INFO]: Member join: %s\n", m.Name)
	}

	if s.Raft != nil {
		if atomic.LoadInt32(&s.Config.BootstrapExpected) != 0 {
			s.tryBootstrap()
		}
	}
}

func (s *Server) nodeLeave(me serf.MemberEvent) {
	for _, m := range me.Members {
		s.logger.Printf("[INFO]: Member leave: %s\n", m.Name)
	}
}

func (s *Server) tryBootstrap() {

	servers := []raft.Server{}
	members := s.Serf.Members()
	for _, member := range members {

		id := member.Tags["id"]
		if id == "" {
			s.logger.Printf("Id not found for member addr: %s", member.Addr.String())
			continue
		}

		raftAddr := member.Tags["raft"]
		if raftAddr == "" {
			s.logger.Printf("Raft addr not found for member: %s", id)
			continue
		}

		peer := raft.Server{
			ID:      raft.ServerID(id),
			Address: raft.ServerAddress(raftAddr),
		}

		servers = append(servers, peer)
	}

	if len(servers) < int(atomic.LoadInt32(&s.Config.BootstrapExpected)) {
		return
	}

	configuration := raft.Configuration{
		Servers: servers,
	}

	if err := s.Raft.BootstrapCluster(configuration).Error(); err != nil {
		s.logger.Printf("Failed to bootstrap cluster: %v\n", err)
	}

	atomic.StoreInt32(&s.Config.BootstrapExpected, 0)
}

func (s *Server) setupRetryJoin() {
	if len(s.Config.RetryJoin) == 0 {
		return
	}

	d := discover.Discover{}
	attempts := 0

	for {
		var addrs []string
		var err error

		for _, addr := range s.Config.RetryJoin {
			if strings.HasPrefix(addr, "provider=") {
				addrs, err = d.Addrs(addr, s.logger)
				if err != nil {
					s.logger.Printf("failed to query go-discover: %s", err)
				}
			} else {
				addrs = append(addrs, addr)
			}
		}

		if _, err := s.Serf.Join(addrs, true); err != nil {
			s.logger.Printf("failed to join %v: %v", addrs, err)
		}

		if attempts == DiscoverBootstrapLimit {
			return
		}

		attempts++
		time.After(10 * time.Second)
	}
}
