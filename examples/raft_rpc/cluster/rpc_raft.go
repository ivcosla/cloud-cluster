package cluster

import (
	"fmt"
	"net"
	"time"

	"github.com/ferranbt/cloud-cluster/cluster"
	"github.com/hashicorp/raft"
)

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

	_, err = conn.Write([]byte{byte(cluster.RPCRaft)})
	if err != nil {
		conn.Close()
		return nil, err
	}
	return conn, err
}
