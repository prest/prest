---
date: 2023-01-22T11:30:00+03:00
title: YugabyteDB
type: homepage
menu:
  yugabytedb:
    parent: "yugabytedb"
weight: 2
---

[YugabyteDB](https://www.yugabyte.com/) is a PostgreSQL-compatible Open-Source Distributed SQL database. It adds horizontal scalability to applications built for PostgreSQL. We can use _p_**REST** works by connecting to any YugabyteDB node.

Start a YugabyteDB cluster with one of the Quick Start methods:
[ YugabyteDB Documentation / Quick Start](https://docs.yugabyte.com/preview/quick-start/)

Start prestd with the PostgreSQL connection string to one YugabyteDB node ( default port is 5433 )

### Node locality

In a public or private cloud, there are multiple ways to scale out the pREST servers with YugabyteDB nodes:

- Start `prestd` with `PREST_PG_URL` set to a cluster service (HA proxy, Kubernetes ClusterIP...) over the YugabyteDB nodes
- Start one `prestd` for each YugabyteDB node, with its local IP
- change the `github.com/jackc/pgx` driver to the cluster-aware one `github.com/yugabyte/pgx/v4`, as in https://docs.yugabyte.com/preview/drivers-orms/go/yb-pgx/, so that it discovers all nodes of the cluster from any node.

With geo-distribution, it is recommended to co-locate the `prestd` server in the same zone as the database node(s) it connects to. This will lower the latency and increase availability.

### Testing

Start a YugabyteDB cluster with one of the Quick Start methods:
[ YugabyteDB Documentation / Quick Start](https://docs.yugabyte.com/preview/quick-start/)

Start `prestd` with the PostgreSQL connection string to one YugabyteDB node ( default port is 5433 )

### Example

Starting a single-node YugabyteDB cluster on Docker:

```sh
docker network create yb-net

docker run -d --hostname yb-tserver-n1 -p 7000:7000 \
  --network yb-net yugabytedb/yugabyte:2.14.6.0-b30 \
  yugabyted start --daemon false --listen yb-tserver-n1
```

Starting `prestd` connecting to this node:

```sh
docker run -d -p 3001:3000 --network yb-net \
  -e PREST_PG_URL=postgres://yugabyte:yugabyte@yb-tserver-n1:5433/yugabyte \
  -e PREST_DEBUG=true \
  prest/prest:v1
```

Creating a view (`yb_servers()` is a table function showing all YugabyteDB nodes in the cluster)

```sh
docker run --rm --network yb-net yugabytedb/yugabyte \
  ysqlsh -h yb-tserver-n1 -c "
   create view yb_servers as select host,zone,region,cloud from yb_servers()
"
```

Querying  this view through the REST API:

```json
curl -i -X GET "http://127.0.0.1:3001/yugabyte/public/yb_servers" -H "Content-Type: application/json"

[{"host": "yb-tserver-n1", "zone": "rack1", "cloud": "cloud1", "region": "datacenter1"}]
```

Starting two more YugabyteDB nodes (`yb-tserver-n2` and `yb-tserver-n3`) to join the previous one (`yb-tserver-n1`):

```sh
docker run -d --hostname yb-tserver-n2 \
  --network yb-net yugabytedb/yugabyte:2.14.6.0-b30 \
  yugabyted start --join yb-tserver-n1 --daemon false --listen yb-tserver-n2
  
docker run -d --hostname yb-tserver-n3 \
  --network yb-net yugabytedb/yugabyte:2.14.6.0-b30 \
  yugabyted start --join yb-tserver-n1 --daemon false --listen yb-tserver-n3
```

Starting one `prestd` for each YugabyteDB node:

```sh
docker run -d -p 3002:3000 --network yb-net \
  -e PREST_PG_URL=postgres://yugabyte:yugabyte@yb-tserver-n2:5433/yugabyte \
  -e PREST_DEBUG=true \
  prest/prest:v1
  
docker run -d -p 3003:3000 --network yb-net \
  -e PREST_PG_URL=postgres://yugabyte:yugabyte@yb-tserver-n3:5433/yugabyte \
  -e PREST_DEBUG=true \
  prest/prest:v1
```

Querying any endpoint to read from the view

```json
curl -i -X GET "http://127.0.0.1:3001/yugabyte/public/yb_servers" -H "Content-Type: application/json"

[{"host": "yb-tserver-n3", "zone": "rack1", "cloud": "cloud1", "region": "datacenter1"}, {"host": "yb-tserver-n2", "zone": "rack1", "cloud": "cloud1", "region": "datacenter1"}, {"host": "yb-tserver-n1", "zone": "rack1", "cloud": "cloud1", "region": "datacenter1"}]

curl -i -X GET "http://127.0.0.1:3003/yugabyte/public/yb_servers" -H "Content-Type: application/json"

[{"host": "yb-tserver-n3", "zone": "rack1", "cloud": "cloud1", "region": "datacenter1"}, {"host": "yb-tserver-n2", "zone": "rack1", "cloud": "cloud1", "region": "datacenter1"}, {"host": "yb-tserver-n1", "zone": "rack1", "cloud": "cloud1", "region": "datacenter1"}]
```

All works as with PostgreSQL, with the additional High Availability and Elasticity provided by YugabyteDB 🚀


