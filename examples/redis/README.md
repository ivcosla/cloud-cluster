This repo creates a redis-cluster of master-slave replicas using the cloud-cluster framework. It will create one master instance and two replicas.

# Install redis

```
$ apt-get install
```

This commands installs 'redis-server' (0.4) and 'redis-cli'. Besides, it starts redis on the background as a systemctl script. To stop it, run:

```
$ sudo systemctl disable redis-server
```

Note that the version of redis installed is not the last stable one. However, it suffices to perform the cluster tests.

# Usage

Deploy a consul agent in dev mode:

```
consul agent -dev
```

Deploy three servers:

```
$ go run main.go --node-name one --bootstrap 3 --serf-port 8001 --rpc-port 5001 --redis-port 6001

$ go run main.go --node-name two --bootstrap 3 --serf-port 8002 --rpc-port 5002 --redis-port 6002

$ go run main.go --node-name three --bootstrap 3 --serf-port 8003 --rpc-port 5003 --redis-port 6003
```

Once all the nodes are active, you can run:

```
$ redis-cli -p 6001
```

to access the instance at port 6001. Then, you can run:

```
$ redis-cli -p 6001 set "a" 1
```

to change the value of "a" to 1 or:

```
$ redis-cli -p 6001 set "a" 2
```

to retrieve the value of "a".

In a redis cluster, read queries (get) can be done to any machine from the cluster. However, insert queries (set) can only be performed on master. If you try to set a value on any of the replicas you will get this message:

```
$ redis-cli -p <port> set "a" 2
(error) READONLY You can't write against a read only slave. 
```

# Internals

Right now, there is no way for the nodes in the raft cluster to know when there is a new master and if they are the secondary instances. Only the master receives those notifications. In this cluster, once the master is selected, it will send a "Status.SlaveOf" message to each slave node so that they can sync with the master.
