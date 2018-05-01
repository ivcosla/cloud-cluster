package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/ferranbt/cloud-cluster/cluster"
	"github.com/hashicorp/nomad/helper/pool"
	"github.com/hashicorp/raft"
)

const (
	raftTimeout = 10 * time.Second
)

func random(min, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
}

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

func newRequest(key, value int) []byte {
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

// --- stream layer
// https://github.com/hashicorp/nomad/blob/master/nomad/raft_rpc.go

type RaftLayer struct {
	addr    net.Addr
	connCh  chan net.Conn
	closeCh chan struct{}
}

func NewRaftLayer(addr net.Addr) *RaftLayer {
	layer := &RaftLayer{
		addr:    addr,
		connCh:  make(chan net.Conn),
		closeCh: make(chan struct{}),
	}
	return layer
}

func (l *RaftLayer) Handoff(c net.Conn) error {
	select {
	case l.connCh <- c:
		return nil
	case <-l.closeCh:
		return fmt.Errorf("Raft RPC layer closed")
	}
}

func (l *RaftLayer) Accept() (net.Conn, error) {
	select {
	case conn := <-l.connCh:
		return conn, nil
	case <-l.closeCh:
		return nil, fmt.Errorf("Raft RPC layer closed")
	}
}

func (l *RaftLayer) Close() error {
	close(l.closeCh)
	return nil
}

func (l *RaftLayer) Addr() net.Addr {
	return l.addr
}

func (l *RaftLayer) Dial(address raft.ServerAddress, timeout time.Duration) (net.Conn, error) {
	conn, err := net.DialTimeout("tcp", string(address), timeout)
	if err != nil {
		return nil, err
	}

	_, err = conn.Write([]byte{byte(pool.RpcRaft)})
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, err
}

// --- rpc server
// https://github.com/hashicorp/nomad/blob/master/nomad/rpc.go

type Server struct {
	logger      *log.Logger
	rpcListener net.Listener
	raftLayer   *RaftLayer
}

func NewRPCServer(logger *log.Logger, raftLayer *RaftLayer, addr *net.TCPAddr) (*Server, error) {
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}

	s := &Server{
		logger:      logger,
		raftLayer:   raftLayer,
		rpcListener: listener,
	}

	return s, nil
}

func (s *Server) listen() {
	for {
		conn, err := s.rpcListener.Accept()
		if err != nil {
			s.logger.Printf("[ERR] RPC: failed to accept RPC conn: %v", err)
			continue
		}

		go s.handleConn(conn)
	}
}

func (s *Server) handleConn(conn net.Conn) {
	buf := make([]byte, 1)
	if _, err := conn.Read(buf); err != nil {
		if err != io.EOF {
			s.logger.Printf("[ERR] RPC: failed to read byte: %v", err)
		}
		conn.Close()
		return
	}

	// Switch on the byte
	switch pool.RPCType(buf[0]) {
	case pool.RpcRaft:
		s.raftLayer.Handoff(conn)
	default:
		s.logger.Printf("[ERR] RPC: unrecognized RPC byte: %v", buf[0])
		conn.Close()
		return
	}
}

// --- config

var rpcAddr string

func readConfig() *cluster.Config {
	config := cluster.DefaultConfig()

	nodeName := flag.String("node-name", "", "Node name")
	serviceName := flag.String("service-name", "", "Service name")
	serfPort := flag.Int("serf-port", 0, "Serf bind port")
	rpcPort := flag.Int("rpc-port", 0, "RPC bind port")
	bootstrap := flag.Int("bootstrap", 1, "Bootstrap expected")

	flag.Parse()

	if nodeName != nil && *nodeName != "" {
		config.NodeName = *nodeName
	}

	if serviceName != nil && *serviceName != "" {
		config.ServiceName = *serviceName
	}

	if serfPort != nil && *serfPort != 0 {
		config.SerfConfig.MemberlistConfig.BindPort = *serfPort
	}

	if rpcPort != nil && *rpcPort != 0 {
		rpcAddr = fmt.Sprintf("127.0.0.1:%d", *rpcPort)
	}

	if bootstrap != nil && *bootstrap != 1 {
		config.BootstrapExpected = int32(*bootstrap)
	}

	return config
}

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	config := readConfig()

	server := cluster.NewServer(config, logger)

	fsm := NewFSM()

	tcpAddr, err := net.ResolveTCPAddr("tcp", rpcAddr)
	if err != nil {
		panic(err)
	}

	raftLayer := NewRaftLayer(tcpAddr)

	rpcServer, err := NewRPCServer(logger, raftLayer, tcpAddr)
	if err != nil {
		panic(err)
	}

	go rpcServer.listen()

	if err := server.SetupStreamRaft(raftLayer, &fsm); err != nil {
		panic(err)
	}

	if err := server.SetupSerf(); err != nil {
		panic(err)
	}

	if err := server.SetupConsul(); err != nil {
		panic(err)
	}

	go func() {
		for {
			select {
			case m := <-server.EventCh:
				fmt.Printf("New member: %s\n", m.String())
			}
		}
	}()

	go func() {
		running := false
		stopCh := make(chan bool)

		for {
			select {
			case isLeader := <-server.LeaderCh:
				if isLeader {
					fmt.Println("Is leader now")
					running = true

					for {
						select {
						case <-time.After(5 * time.Second):
							data := newRequest(random(1, 100), random(1, 100))

							if err := server.Apply(data, raftTimeout).Error(); err != nil {
								panic(err)
							}

						case <-stopCh:
							return
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
	}()

	done := make(chan bool)
	<-done
}
