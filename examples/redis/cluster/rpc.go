package cluster

import (
	"fmt"
	"io"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"time"
)

type RPCType byte

const (
	rpcCluster   RPCType = 0x01
	rpcRaft              = 0x02
	rpcMultiplex         = 0x03
)

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
	fmt.Println("Conn")
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
	case rpcCluster:
		fmt.Println("Cluster mode")
		s.handleClusterConn(conn)
	case rpcRaft:
		s.raftLayer.Handoff(conn)
	default:
		s.logger.Printf("[ERR] RPC: unrecognized RPC byte: %v", buf[0])
		conn.Close()
		return
	}
}

func (s *Server) handleClusterConn(conn net.Conn) {
	codec := jsonrpc.NewServerCodec(conn)
	defer conn.Close()
	if err := s.rpcServer.ServeRequest(codec); err != nil {
		s.logger.Printf("[ERR] RPC error: %v (%v)", err, conn)
	}
}

func (s *Server) forward(method string, args interface{}, reply interface{}) (bool, error) {
	// Find the leader
	isLeader, remoteServer := s.getLeader()

	// Handle the case we are the leader
	if isLeader {
		return false, nil
	}

	// Handle the case of a known leader
	if remoteServer != "" {
		err := s.rpc(remoteServer, method, args, reply)
		return true, err
	}

	return true, fmt.Errorf("No leader found")
}

// rpc is used to forward an RPC call to another peer
func (s *Server) rpc(server string, method string, args interface{}, reply interface{}) error {
	conn, err := net.DialTimeout("tcp", server, 10*time.Second)
	if err != nil {
		return err
	}

	if _, err := conn.Write([]byte{byte(rpcCluster)}); err != nil {
		conn.Close()
		return err
	}

	codec := jsonrpc.NewClientCodec(conn)
	client := rpc.NewClientWithCodec(codec)
	return client.Call(method, args, reply)
}

func (s *Server) getLeader() (bool, string) {
	// Check if we are the leader
	if s.IsLeader() {
		return true, ""
	}

	// Get the leader
	leader := s.cluster.Raft.Leader()
	if leader == "" {
		return false, ""
	}

	return false, string(leader)
}
