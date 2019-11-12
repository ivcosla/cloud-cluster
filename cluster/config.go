package cluster

import (
	"io"
	"os"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/raft"
	"github.com/hashicorp/serf/serf"
)

// RPCType represents different types of rpc to be used
type RPCType byte

const (
	// RPCRaft is the Raft rpc type
	RPCRaft RPCType = 0x02
)

// Config for the cluster server
type Config struct {
	NodeName    string
	ServiceName string

	Tags map[string]string

	RetryJoin []string

	SerfConfig   *serf.Config
	ConsulConfig *consul.Config

	RaftConfig        *raft.Config
	BootstrapExpected int32

	LogOutput io.Writer
}

// DefaultConfig returns a cluster server config with default values
func DefaultConfig() *Config {
	c := &Config{
		NodeName:          "cluster",
		ServiceName:       "cluster",
		Tags:              map[string]string{},
		RetryJoin:         []string{},
		SerfConfig:        serf.DefaultConfig(),
		ConsulConfig:      consul.DefaultConfig(),
		RaftConfig:        raft.DefaultConfig(),
		BootstrapExpected: 1,
		LogOutput:         os.Stderr,
	}

	return c
}
