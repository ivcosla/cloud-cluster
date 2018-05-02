package cluster

import (
	"io"
	"log"
	"net"
)

// --- rpc server
// https://github.com/hashicorp/nomad/blob/master/nomad/rpc.go

type RPCType byte

const (
	rpcRaft RPCType = 0x01
)

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

func (s *Server) Listen() {
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
	switch RPCType(buf[0]) {
	case rpcRaft:
		s.raftLayer.Handoff(conn)
	default:
		s.logger.Printf("[ERR] RPC: unrecognized RPC byte: %v", buf[0])
		conn.Close()
		return
	}
}
