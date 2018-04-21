
This example uses Consul, Serf and Raft to deploy a 3 nodes cluster. The FSM implements a simple key value store. Every 5 seconds, the leader will update the state with random values.

# Usage

Deploy a consul agent in dev mode:

```
consul agent -dev
```

Deploy three servers:

```
$ go run main.go --node-name one --bootstrap 3 --serf-port 8000

$ go run main.go --node-name two --bootstrap 3 --serf-port 8001 --raft-port 5001

$ go run main.go --node-name three --bootstrap 3 --serf-port 8002 --raft-port 5002
```
