# devops-id ID Generation Service Integration Guide

`devops-id` is a gRPC distributed ID service based on the Snowflake algorithm. The service isolates worker-ID allocation spaces in etcd by `leaf_name + biz_tag`; a worker is allocated for a business tag only the first time that `biz_tag` is requested.

## Integration Overview

| Item | Description |
| --- | --- |
| Protocol | gRPC / protobuf |
| Service name | `idgen.IdGen` |
| Default listen | `:9990` |
| Methods | `Next`, `BatchNext` |
| ID type | `int64` |
| Health check | `grpc.health.v1.Health/Check` |
| Transport security | plaintext gRPC for now; deploy inside a trusted intranet |

## ID Semantics

- Within a single `biz_tag`, IDs are unique and roughly increasing, but not guaranteed to be contiguous.
- The ID spaces of different `biz_tag` values are fully independent, so identical IDs may appear across them.
- `biz_tag` does not participate in Snowflake bit arithmetic; together with `leaf_name` it determines the etcd slot and local worker cache namespace.
- When multiple clients fetch IDs concurrently, IDs obtained by different clients may interleave.
- Service restarts or unconsumed client prefetch buffers create gaps; do not treat IDs as a gap-free sequence.
- Use IDs only as business identifiers; do not parse business meaning out of an ID.

## Worker Allocation

The Snowflake ID layout is:

```text
| 1 bit sign | 41 bit ms timestamp | 10 bit worker ID | 12 bit sequence |
```

Worker ID allocation is isolated along the following dimensions:

```text
idgen/snowflake/{leaf_name}/{biz_tag}/
```

For example, with `leaf_name=devops-id` and `biz_tag=user`, a service instance claims a slot under `idgen/snowflake/devops-id/user/`. Other `biz_tag` values such as `role` use independent slot prefixes and therefore have independent worker-ID spaces.

The local worker cache path is:

```text
{worker_cache}/{leaf_name}/{biz_tag}/worker_id
```

No worker is allocated at service startup. Only when a `biz_tag` receives its first ID request does `devops-id` create the generator for that tag and allocate a worker ID via etcd. Subsequent requests for the same `biz_tag` reuse that generator.

`leaf_name` and `biz_tag` must be safe path components: non-empty, free of leading/trailing whitespace, and containing none of `.`, `..`, `/` or `\`.

## gRPC Contract

Proto source: [`proto/idgen.proto`](./proto/idgen.proto).

```protobuf
service IdGen {
  rpc Next(NextRequest) returns (NextReply);
  rpc BatchNext(BatchNextRequest) returns (BatchNextReply);
}

message NextRequest {
  string biz_tag = 1;
}

message NextReply {
  int64 id = 1;
}

message BatchNextRequest {
  string biz_tag = 1;
  int32 count = 2;
}

message BatchNextReply {
  repeated int64 ids = 1;
}
```

### Methods

| Method | Full method | Request | Response | Purpose |
| --- | --- | --- | --- | --- |
| `Next` | `/idgen.IdGen/Next` | `biz_tag` | `id` | Fetch a single ID |
| `BatchNext` | `/idgen.IdGen/BatchNext` | `biz_tag`, `count` | `ids` | Atomically fetch a batch of IDs for client prefetch buffering |

### Parameter Constraints

- `biz_tag` must be a non-empty business identifier usable as a path component; prefer stable singular identifiers such as `user` or `order`.
- `BatchNext.count` ranges over `1..100000`.
- The list returned by `BatchNext` is monotonically increasing within a single response.
- Use a modest batch size in production (e.g. 100); do not treat the maximum as a normal setting.

## Errors and Retries

| gRPC status | Meaning | Recommendation |
| --- | --- | --- |
| `InvalidArgument` | invalid `biz_tag` or out-of-range `count` | Fix the request parameters; do not retry as-is |
| `Unavailable` | etcd unavailable, clock rollback beyond threshold, worker lease expired, service unreachable | Retry with exponential backoff |
| `Internal` | server-side internal error | Log the error and check the server logs; do not retry blindly |

The retry semantics are "at most extra IDs are consumed; no duplicate IDs are produced within the same `biz_tag`". If a request reached the server but the response was lost, those IDs may already be allocated without the caller receiving them; retrying yields new IDs. Do not cache the IDs of failed requests and reuse them at the business layer.

## Go Integration

### Recommended: Use the `idclient` Prefetch Buffer

`pkg/idclient` wraps `BatchNext` prefetching, `biz_tag`-isolated local buffers, low-watermark async replenishment and health checks. For a single `biz_tag`, locally issued IDs increase in order.

```go
package main

import (
	"context"
	"fmt"
	"time"

	grpcinfra "devops-infra/grpc"
	"devops-id/pkg/idclient"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpcinfra.Dial(ctx, "127.0.0.1:9990", 2*time.Second)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := idclient.New(conn, idclient.BufferConfig{
		Batch:       100,
		LowWater:    0.2,
		CallTimeout: 3 * time.Second,
	})
	if err := client.Ping(ctx); err != nil {
		panic(err)
	}

	id, err := client.Next(context.Background(), "user")
	if err != nil {
		panic(err)
	}
	fmt.Println(id)
}
```

An `idclient.Client` can be shared by multiple goroutines and maintains independent buffers per `biz_tag` internally.

### Configuration Recommendations

| Setting | Suggested value | Description |
| --- | --- | --- |
| `batch` | `1..100` | Prefetch size; larger batches mean fewer RPCs but may waste more IDs on process exit |
| `low_water` | `0.1..0.3` | Async replenishment triggers when the remaining ratio drops below this value |
| `dial_timeout` | `1s..5s` | Timeout for establishing the client connection |
| `call_timeout` | `1s..5s` | Timeout for a single `BatchNext` call |

### Direct gRPC Stub

If you do not use the prefetch buffer, you can use the generated `idgenpb` client directly:

```go
package main

import (
	"context"
	"fmt"
	"time"

	grpcinfra "devops-infra/grpc"
	"devops-id/pkg/idgenpb"
)

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, err := grpcinfra.Dial(ctx, "127.0.0.1:9990", 2*time.Second)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	client := idgenpb.NewIdGenClient(conn)
	next, err := client.Next(context.Background(), &idgenpb.NextRequest{BizTag: "user"})
	if err != nil {
		panic(err)
	}
	fmt.Println(next.GetId())
}
```

## GORM Model Integration

`pkg/idgorm` provides a shared ID-assignment seam for the GORM `BeforeCreate` scenario. It depends only on a minimal generator interface:

```go
type Generator interface {
	Next(ctx context.Context, bizTag string) (int64, error)
}
```

`idclient.Client` already satisfies this interface.

```go
package model

import (
	"gorm.io/gorm"

	"devops-id/pkg/idgorm"
)

var idHook = idgorm.New(nil, nil)

func SetIDGenerator(generator idgorm.Generator) {
	idHook.SetGenerator(generator)
}

type Order struct {
	ID int64 `gorm:"primaryKey;autoIncrement:false"`
}

func (Order) TableName() string { return "t_order" }

func (m *Order) BeforeCreate(tx *gorm.DB) error {
	return idHook.Assign(tx, &m.ID)
}
```

`idHook.Assign` preserves non-zero explicit IDs and only generates IDs for zero-valued primary keys. Snowflake workers are allocated automatically on first use by `leaf_name + biz_tag`; no database pre-registration is needed.

By default, `biz_tag` is derived from the GORM model's Go type name as `snake_case` (e.g. `OrderItem` -> `order_item`), so new regular models need no table-name mapping. To make several models share one ID space, or when the type name is unsuitable as a business identifier, implement `idgorm.BizTagProvider` to return a stable tag explicitly; explicit tags are recommended when the ID space must survive code renames. The legacy table-name-to-tag mapping remains available via `idgorm.New(generator, map[string]string{...})` for compatibility with existing tags or special cases.

## Quick Verification with grpcurl

gRPC reflection is enabled by default in the development environment:

```bash
# list services
grpcurl -plaintext 127.0.0.1:9990 list

# health check
grpcurl -plaintext -d '{}' 127.0.0.1:9990 grpc.health.v1.Health/Check

# fetch a single ID
grpcurl -plaintext -d '{"biz_tag":"user"}' 127.0.0.1:9990 idgen.IdGen/Next

# fetch a batch of IDs
grpcurl -plaintext -d '{"biz_tag":"user","count":10}' 127.0.0.1:9990 idgen.IdGen/BatchNext
```

Protobuf JSON emits `int64` as a string:

```json
{
  "id": "7306137248161302528"
}
```

## Integration from Other Languages

1. Copy or reference [`proto/idgen.proto`](./proto/idgen.proto).
2. Use the protoc plugin for your language to generate the `idgen.IdGen` client.
3. Call `/idgen.IdGen/Next` or `/idgen.IdGen/BatchNext`.
4. Use the standard gRPC health check `/grpc.health.v1.Health/Check` to determine whether the service is `SERVING`.
5. Expose `int64` IDs as strings in external APIs and JSON data structures.

## Code Structure (DDD Layers)

```text
internal/
├── cmd/                # cobra entrypoint: config/log/observability init + server lifecycle orchestration
├── config/             # config structs (reusing devops-infra/config sections)
├── app/                # Wire dependency injection (wire.go / providers.go / wire_gen.go)
├── domain/idgen/       # domain layer: Snowflake bit layout, tag generators, Manager,
│                       # domain errors and ports (LeaseLifecycle / SlotCoordinator / WorkerCache)
├── application/idgen/  # application layer: Next / BatchNext use-case service
├── infrastructure/     # infrastructure layer: etcdcoord (lease + slot coordination adapter),
│                       # workerfile (local worker cache files)
└── interfaces/grpc/    # interface layer: IdGen gRPC handler (parameter validation, error mapping)
```

Dependency direction: `interfaces -> application -> domain <- infrastructure`. The domain layer depends only on the standard library; etcd coordination and disk caching are injected through ports. The generic etcd lease / slot coordination implementation lives in the shared `devops-infra/etcd` library.

Regenerate the injector after dependency graph changes:

```bash
go generate -tags=wireinject ./internal/app
```

## Integration Checklist

- [ ] Service address and port confirmed and reachable.
- [ ] A stable `biz_tag` determined for each kind of business object.
- [ ] Understood that uniqueness is scoped by `biz_tag` and duplicates are allowed across different `biz_tag` values.
- [ ] IDs treated as unique identifiers only, with no reliance on contiguity.
- [ ] `BatchNext.count` set to the smallest reasonable value the business needs.
- [ ] Exponential backoff retries used for `Unavailable`; parameter errors fixed before retrying.
- [ ] `int64` IDs serialized as strings in external APIs.
- [ ] Plaintext gRPC deployed only inside a trusted intranet, with TLS/mTLS added later as needed.
