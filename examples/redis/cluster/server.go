package cluster

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"net/rpc"
	"os/exec"
	"strconv"
	"strings"

	ccluster "github.com/ferranbt/cloud-cluster/cluster"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"

	example "github.com/ferranbt/cloud-cluster/examples/raft_rpc/cluster"
)

const (
	redisCli    = "redis-cli"
	redisServer = "redis-server"
)

var (
	DefaultRPCAddr  = &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 5000}
	DefaultHTTPAddr = &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: 9000}
)

type Config struct {
	RPCAddr   *net.TCPAddr
	HTTPAddr  *net.TCPAddr
	Cluster   *ccluster.Config
	RedisPort int
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

	closeRedis func()
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

	server.cluster = ccluster.NewServer(config.Cluster, logger)

	server.raftLayer = NewRaftLayer(config.RPCAddr)

	go server.Listen()

	fsm := example.NewFSM()

	err := server.cluster.SetupStreamRaft(server.raftLayer, &fsm)
	if err != nil {
		return nil, err
	}

	if err := server.cluster.SetupSerf(); err != nil {
		return nil, err
	}

	if err := server.cluster.SetupConsul(); err != nil {
		return nil, err
	}

	// setup redis
	server.closeRedis, err = runCmdBackground(redisServer, "--port", strconv.Itoa(config.RedisPort))
	if err != nil {
		return nil, err
	}

	go server.handleLeader()
	return server, nil
}

// Close closes all the tasks
func (s *Server) Close() error {
	if s.closeRedis != nil {
		s.closeRedis()
	}
	return nil
}

func (s *Server) slaveOf(masterPort int) {
	out, err := runCmd(redisCli, "-p", strconv.Itoa(s.config.RedisPort), "slaveof", "localhost", strconv.Itoa(masterPort))
	if err != nil {
		panic(err)
	}

	fmt.Println("-- out --")
	fmt.Println(out)
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

func (s *Server) handleLeader() {
	running := false
	stopCh := make(chan bool)

	for {
		select {
		case isLeader := <-s.cluster.LeaderCh:
			if isLeader {
				running = true

				// Send an RPC message to all the nodes
				for _, addr := range s.getNodes() {

					var peers []string
					if err := s.rpc(addr, "Status.Peers", nil, &peers); err != nil {
						panic(err)
					}

					fmt.Println("-- peers --")
					fmt.Println(peers)

					var out bool
					if err := s.rpc(addr, "Status.SlaveOf", s.config.RedisPort, &out); err != nil {
						panic(err)
					}
				}

			} else {
				if running {
					stopCh <- true
				}
				fmt.Println("Lost leadership")
			}
		}
	}
}

func (s *Server) getNodes() (reply []string) {
	future := s.cluster.Raft.GetConfiguration()
	if err := future.Error(); err != nil {
		panic(err)
	}

	for _, server := range future.Configuration().Servers {
		if server.Address != s.cluster.Raft.Leader() {
			reply = append(reply, string(server.Address))
		}
	}
	return
}

func runCmd(path string, args ...string) (string, error) {
	var out, out1 bytes.Buffer
	cmd := exec.Command(path, args...)
	cmd.Stdout = &out
	cmd.Stderr = &out1
	if err := cmd.Run(); err != nil {
		return "", err
	}
	if b := out1.Bytes(); len(b) != 0 {
		return "", fmt.Errorf(string(b))
	}
	return strings.TrimSpace(string(out.Bytes())), nil
}

func runCmdBackground(path string, args ...string) (func(), error) {
	cmd := exec.Command(path, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil

	if err := cmd.Start(); err != nil {
		return nil, err
	}

	close := func() {
		if err := cmd.Process.Kill(); err != nil {
			panic(err)
		}
		cmd.Wait()
	}
	return close, nil
}
