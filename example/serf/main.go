package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ferranbt/cloud-cluster/cluster"
)

// --- config

func readConfig() *cluster.Config {
	config := cluster.DefaultConfig()

	nodeName := flag.String("node-name", "", "Node name")
	serviceName := flag.String("service-name", "", "Service name")
	serfPort := flag.Int("serf-port", 0, "Serf bind port")

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

	return config
}

func main() {
	logger := log.New(os.Stderr, "", log.LstdFlags)
	config := readConfig()

	server := cluster.NewServer(config, logger)

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

	done := make(chan bool)
	<-done
}
