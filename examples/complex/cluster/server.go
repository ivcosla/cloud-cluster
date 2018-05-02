package cluster

import (
	"log"
	"net"
	"net/rpc"

	ccluster "github.com/ferranbt/cloud-cluster/cluster"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

var (
	DefaultRPCAddr  = &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5000}
	DefaultHTTPAddr = &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9000}
)

type Config struct {
	RPCAddr  *net.TCPAddr
	HTTPAddr *net.TCPAddr
	Cluster  *ccluster.Config
}

func DefaultConfig() *Config {
	c := &Config{
		RPCAddr:  DefaultRPCAddr,
		HTTPAddr: DefaultHTTPAddr,
		Cluster:  ccluster.DefaultConfig(),
	}

	return c
}

type Server struct {
	config      *Config
	cluster     *ccluster.Server
	logger      *log.Logger
	rpcListener net.Listener
	raftLayer   *RaftLayer
	rpcServer   *rpc.Server
	endpoints   endpoints
	http        *HTTPServer
}

type endpoints struct {
	Status *Status
}

func NewServer(config *Config, logger *log.Logger) (*Server, error) {
	server := &Server{
		config:    config,
		logger:    logger,
		rpcServer: rpc.NewServer(),
	}

	if err := server.setupRPC(); err != nil {
		return nil, err
	}

	http, err := NewHttpServer(server)
	if err != nil {
		return nil, err
	}
	server.http = http

	server.cluster = ccluster.NewServer(config.Cluster, logger)

	server.raftLayer = NewRaftLayer(config.RPCAddr)

	go server.Listen()

	fsm := NewFSM()

	if err := server.cluster.SetupStreamRaft(server.raftLayer, &fsm); err != nil {
		return nil, err
	}

	if err := server.cluster.SetupSerf(); err != nil {
		return nil, err
	}

	if err := server.cluster.SetupConsul(); err != nil {
		return nil, err
	}

	return server, nil
}

func (s *Server) setupRPC() error {
	s.endpoints.Status = &Status{s}

	s.rpcServer.Register(s.endpoints.Status)

	var err error
	s.rpcListener, err = net.ListenTCP("tcp", s.config.RPCAddr)
	if err != nil {
		return err
	}

	return nil
}

func (s *Server) IsLeader() bool {
	return s.cluster.Raft.State() == raft.Leader
}

func (s *Server) Join(peers []string) (int, error) {
	return s.cluster.Serf.Join(peers, true)
}

func (s *Server) Members() []serf.Member {
	return s.cluster.Serf.Members()
}
