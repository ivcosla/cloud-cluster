package main

import (
	"flag"
	"log"
	"net"
	"os"

	"github.com/ferranbt/cloud-cluster/examples/complex/cluster"
)

func readConfig() *cluster.Config {
	config := cluster.DefaultConfig()

	nodeName := flag.String("node-name", "", "Node name")
	serviceName := flag.String("service-name", "", "Service name")
	serfPort := flag.Int("serf-port", 0, "Serf bind port")
	rpcPort := flag.Int("rpc-port", 0, "RPC bind port")
	httpPort := flag.Int("http-port", 0, "RPC bind port")
	bootstrap := flag.Int("bootstrap", 1, "Bootstrap expected")

	flag.Parse()

	if nodeName != nil && *nodeName != "" {
		config.Cluster.NodeName = *nodeName
	}

	if serviceName != nil && *serviceName != "" {
		config.Cluster.ServiceName = *serviceName
	}

	if serfPort != nil && *serfPort != 0 {
		config.Cluster.SerfConfig.MemberlistConfig.BindPort = *serfPort
	}

	if rpcPort != nil && *rpcPort != 0 {
		config.RPCAddr = &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: *rpcPort}
	}

	if httpPort != nil && *httpPort != 0 {
		config.HTTPAddr = &net.TCPAddr{IP: net.ParseIP("127.0.0.1"), Port: *httpPort}
	}

	if bootstrap != nil && *bootstrap != 1 {
		config.Cluster.BootstrapExpected = int32(*bootstrap)
	}

	return config
}

func main() {

	logger := log.New(os.Stderr, "", log.LstdFlags)
	config := readConfig()

	_, err := cluster.NewServer(config, logger)
	if err != nil {
		panic(err)
	}

	done := make(chan bool)
	<-done
}
