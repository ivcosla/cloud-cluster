package cluster

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/hashicorp/raft"
)

type RaftConfig struct {
	*raft.Config

	BindAddr string
	BindPort int

	BootstrapExpected int32
}

func DefaultRaftConfig() *RaftConfig {
	c := &RaftConfig{
		raft.DefaultConfig(),
		"127.0.0.1", // BindAddr
		5000,        // BindPort,
		1,           // BootstrapExpected
	}

	return c
}

func (s *Server) SetupRaft(fsm *raft.FSM) error {
	raftBind := fmt.Sprintf("%s:%d", s.config.RaftConfig.BindAddr, s.config.RaftConfig.BindPort)

	addr, err := net.ResolveTCPAddr("tcp", raftBind)
	if err != nil {
		return fmt.Errorf("Cannot resolve raft bind addr %s: %v", raftBind, err)
	}

	trans, err := raft.NewTCPTransport(raftBind, addr, 5, 5*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("Failed to create tcp transport: %v", err)
	}

	s.config.RaftConfig.LogOutput = s.config.LogOutput
	s.config.RaftConfig.LocalID = raft.ServerID(s.config.NodeName)

	store := raft.NewInmemStore()
	stable := store
	log := store
	snap := raft.NewDiscardSnapshotStore()

	s.config.RaftConfig.Config.NotifyCh = s.LeaderCh

	if s.config.RaftConfig.BootstrapExpected == 1 {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      s.config.RaftConfig.Config.LocalID,
					Address: trans.LocalAddr(),
				},
			},
		}

		if err := raft.BootstrapCluster(s.config.RaftConfig.Config, log, stable, snap, trans, configuration); err != nil {
			return fmt.Errorf("Failed to bootstrap initial cluster: %v", err)
		}
	}

	client, err := raft.NewRaft(s.config.RaftConfig.Config, *fsm, log, stable, snap, trans)
	if err != nil {
		return fmt.Errorf("Failed to start raft: %v", err)
	}

	s.raft = client

	go s.reconcile()
	return nil
}

func (s *Server) reconcile() {
	for {
		select {
		case member := <-s.reconcileCh:
			fmt.Println("reconcile")

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

			addFuture := s.raft.AddVoter(raft.ServerID(id), raft.ServerAddress(raftAddr), 0, 0)

			if err := addFuture.Error(); err != nil {
				s.logger.Printf("Failed to add peer %s to raft", id)
			}
		}
	}
}
