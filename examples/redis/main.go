package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ferranbt/cloud-cluster/examples/redis/cluster"
)

func readConfig() *cluster.Config {
	config := cluster.DefaultConfig()

	nodeName := flag.String("node-name", "", "Node name")
	serviceName := flag.String("service-name", "", "Service name")
	serfPort := flag.Int("serf-port", 0, "Serf bind port")
	rpcPort := flag.Int("rpc-port", 0, "RPC bind port")
	httpPort := flag.Int("http-port", 0, "RPC bind port")
	redisPort := flag.Int("redis-port", 0, "Redis server bind port")
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

	if redisPort != nil && *redisPort != 0 {
		config.RedisPort = *redisPort
	}
	return config
}

func main() {

	logger := log.New(os.Stderr, "", log.LstdFlags)
	config := readConfig()

	server, err := cluster.NewServer(config, logger)
	if err != nil {
		panic(err)
	}

	handleSignals(server.Close)
}

const gracefulTimeout = 5 * time.Second

func handleSignals(closeFn func() error) int {
	signalCh := make(chan os.Signal, 4)
	signal.Notify(signalCh, os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)

	var sig os.Signal
	select {
	case sig = <-signalCh:
	}

	fmt.Printf("Caught signal: %v\n", sig)
	fmt.Println("Gracefully shutting down agent...")

	gracefulCh := make(chan struct{})
	go func() {
		if err := closeFn(); err != nil {
			return
		}
		close(gracefulCh)
	}()

	select {
	case <-signalCh:
		return 1
	case <-time.After(gracefulTimeout):
		return 1
	case <-gracefulCh:
		return 0
	}
}
