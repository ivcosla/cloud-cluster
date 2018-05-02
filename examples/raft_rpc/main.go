package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"time"

	"github.com/ferranbt/cloud-cluster/cluster"
	example "github.com/ferranbt/cloud-cluster/examples/raft_rpc/cluster"
)

const (
	raftTimeout = 10 * time.Second
)

func random(min, max int) int {
	rand.Seed(time.Now().Unix())
	return rand.Intn(max-min) + min
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

	fsm := example.NewFSM()

	tcpAddr, err := net.ResolveTCPAddr("tcp", rpcAddr)
	if err != nil {
		panic(err)
	}

	raftLayer := example.NewRaftLayer(tcpAddr)

	rpcServer, err := example.NewRPCServer(logger, raftLayer, tcpAddr)
	if err != nil {
		panic(err)
	}

	go rpcServer.Listen()

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
							data := example.NewRequest(random(1, 100), random(1, 100))

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
