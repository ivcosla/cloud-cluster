# Cloud-Cluster

This repo contains a set of utilities to deploy and bootstrap clusters using cloud tools developed by [Hashicorp](https://www.hashicorp.com/). It is based on the framework used in [Nomad](https://www.nomadproject.io/).

The nodes of the cluster use [Consul](https://www.consul.io/) to register themselves (with a specific service name) and query for other nodes that belong to the cluster. Then, new nodes found on Consul are piped into [Serf](https://www.serf.io/) to check for membership and failure detection. It can also use [Raft](https://github.com/hashicorp/raft) to provide a shared state between the servers.

## TODO

- Use consul connect to proxy communications.
- Docker with redis.
- Can new instances become raft-leaders? They should not if their state is not fully synced (Nomad?).
