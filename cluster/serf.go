package cluster

import (
	"sync/atomic"

	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

func (s *Server) SetupSerf() error {
	conf := s.config.SerfConfig
	conf.Init()

	conf.Tags["id"] = s.config.NodeName

	for k, v := range s.config.Tags {
		conf.Tags[k] = v
	}

	conf.NodeName = s.config.NodeName
	conf.MemberlistConfig.LogOutput = s.config.LogOutput
	conf.LogOutput = s.config.LogOutput
	conf.EventCh = s.localEventCh

	client, err := serf.Create(conf)
	if err != nil {
		return err
	}

	s.serf = client
	go s.eventHandler()

	return nil
}

func (s *Server) eventHandler() {
	for {
		select {
		case e := <-s.localEventCh:

			select {
			case s.EventCh <- e:
			default:
			}

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
}

func (s *Server) localMemberEvent(me serf.MemberEvent) {
	if s.raft == nil {
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

	if s.raft != nil {
		if atomic.LoadInt32(&s.config.BootstrapExpected) != 0 {
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
	members := s.serf.Members()
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

	if len(servers) < int(atomic.LoadInt32(&s.config.BootstrapExpected)) {
		return
	}

	configuration := raft.Configuration{
		Servers: servers,
	}

	if err := s.raft.BootstrapCluster(configuration).Error(); err != nil {
		s.logger.Printf("Failed to bootstrap cluster: %v\n", err)
	}

	atomic.StoreInt32(&s.config.BootstrapExpected, 0)
}
