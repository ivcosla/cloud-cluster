package cluster

import (
	"fmt"
	"net"
	"os"
	"time"

	"github.com/hashicorp/raft"
)

const (
	raftTimeout = 10 * time.Second
)

func (s *Server) SetupStreamRaft(layer raft.StreamLayer, fsm *raft.FSM) error {
	trans := raft.NewNetworkTransport(layer, 3, raftTimeout, s.Config.RaftConfig.LogOutput)
	return s.setupRaft(trans, fsm)
}

func (s *Server) SetupTCPRaft(bindAddr string, fsm *raft.FSM) error {
	addr, err := net.ResolveTCPAddr("tcp", bindAddr)
	if err != nil {
		return fmt.Errorf("Cannot resolve raft bind addr %s: %v", bindAddr, err)
	}

	trans, err := raft.NewTCPTransport(bindAddr, addr, 5, 5*time.Second, os.Stderr)
	if err != nil {
		return fmt.Errorf("Failed to create tcp transport: %v", err)
	}

	return s.setupRaft(trans, fsm)
}

func (s *Server) setupRaft(trans raft.Transport, fsm *raft.FSM) error {
	s.Config.Tags["raft"] = string(trans.LocalAddr())

	s.Config.RaftConfig.LogOutput = s.Config.LogOutput
	s.Config.RaftConfig.LocalID = raft.ServerID(s.Config.NodeName)

	store := raft.NewInmemStore()
	stable := store
	log := store
	snap := raft.NewDiscardSnapshotStore()

	s.Config.RaftConfig.NotifyCh = s.LeaderCh

	if s.Config.BootstrapExpected == 1 {
		configuration := raft.Configuration{
			Servers: []raft.Server{
				{
					ID:      s.Config.RaftConfig.LocalID,
					Address: trans.LocalAddr(),
				},
			},
		}

		if err := raft.BootstrapCluster(s.Config.RaftConfig, log, stable, snap, trans, configuration); err != nil {
			return fmt.Errorf("Failed to bootstrap initial cluster: %v", err)
		}
	}

	client, err := raft.NewRaft(s.Config.RaftConfig, *fsm, log, stable, snap, trans)
	if err != nil {
		return fmt.Errorf("Failed to start raft: %v", err)
	}

	s.Raft = client

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

			addFuture := s.Raft.AddVoter(raft.ServerID(id), raft.ServerAddress(raftAddr), 0, 0)

			if err := addFuture.Error(); err != nil {
				s.logger.Printf("Failed to add peer %s to raft", id)
			}
		}
	}
}
