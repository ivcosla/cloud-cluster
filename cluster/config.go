package cluster

import (
	"io"
	"os"

	consul "github.com/hashicorp/consul/api"
	"github.com/hashicorp/serf/serf"
)

type Config struct {
	NodeName    string
	ServiceName string
	Tag         string

	SerfConfig   *serf.Config
	ConsulConfig *consul.Config
	RaftConfig   *RaftConfig

	LogOutput io.Writer
}

func DefaultConfig() *Config {
	c := &Config{
		NodeName:     "cluster",
		ServiceName:  "cluster",
		Tag:          "cluster",
		SerfConfig:   serf.DefaultConfig(),
		ConsulConfig: consul.DefaultConfig(),
		RaftConfig:   DefaultRaftConfig(),
		LogOutput:    os.Stderr,
	}

	return c
}
