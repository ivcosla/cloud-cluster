
# Usage

Deploy a consul agent in dev mode:

```
consul agent -dev
```

Deploy three servers:

```
$ go run main.go --node-name one --bootstrap 3 --serf-port 8000 --rpc-port 5000 --http-port 9000

$ go run main.go --node-name two --bootstrap 3 --serf-port 8001 --rpc-port 5001 --http-port 9001

$ go run main.go --node-name three --bootstrap 3 --serf-port 8002 --rpc-port 5002 --http-port 9002
```
