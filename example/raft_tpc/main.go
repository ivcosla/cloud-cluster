package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/ferranbt/cloud-cluster/cluster"
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

// --- config

var raftAddr string

func readConfig() *cluster.Config {
	config := cluster.DefaultConfig()

	nodeName := flag.String("node-name", "", "Node name")
	serviceName := flag.String("service-name", "", "Service name")
	serfPort := flag.Int("serf-port", 0, "Serf bind port")
	raftPort := flag.Int("raft-port", 0, "Raft bind port")
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

	if raftPort != nil && *raftPort != 0 {
		raftAddr = fmt.Sprintf("127.0.0.1:%d", *raftPort)
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

	if err := server.SetupTCPRaft(raftAddr, &fsm); err != nil {
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
