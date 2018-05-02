
This example uses Consul and Serf to deploy a 3 nodes cluster.

# Usage

Deploy a consul agent in dev mode:

```
consul agent -dev
```

Deploy three servers:

```
$ go run main.go --node-name one --serf-port 8000

$ go run main.go --node-name two --serf-port 8001

$ go run main.go --node-name three --serf-port 8002
```
