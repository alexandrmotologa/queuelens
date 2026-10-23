# QueueLens: Master Implementation Plan

> Universal background job and task queue observability studio for Redis and Valkey (BullMQ, Celery, Asynq, Sidekiq).
> Real-time state machine tracking, worker stall detection, and Dead-Letter Queue (DLQ) studio with error clustering and mass replay.

---

## 1. Architecture Overview

Modern distributed systems rely on Redis and Valkey as the message broker for background jobs. In practice, backend teams encounter four operational challenges:
1. Multi-language polyglot stacks mix queue systems (BullMQ in TypeScript, Celery in Python, Asynq in Go, Sidekiq in Ruby) without a single pane of glass.
2. Silent worker stalls happen when workers crash or get killed by OOM mid-execution, leaving jobs locked in `active` state until TTL expiration.
3. Retry cascades overwhelm downstream databases when failed jobs retry repeatedly without backoff.
4. Dead-letter queue analysis requires raw Redis CLI commands without error grouping or payload remediation.

QueueLens runs as a standalone Go binary with an embedded React 19 interface. It connects to standalone Redis, Valkey, or Redis Sentinel/Cluster topologies without requiring plugins or intrusive agents inside worker processes.

```
+-----------------------------------------------------------------------------------+
|                                     QUEUELENS                                     |
+-----------------------------------------+-----------------------------------------+
                                          |
        +---------------------------------+---------------------------------+
        |                                                                   |
+-------v-------------------------+                 +-----------------------v-------+
|   Redis Auto-Discovery Engine   |                 |    Queue State Machine Engine |
|  * BullMQ (bull:<q>:wait)       |                 |  * Waiting, Active, Delayed   |
|  * Celery (celery, task-meta)   |                 |  * Failed (DLQ), Completed    |
|  * Asynq (asynq:{<q>}:pending)  |                 |  * Throughput (Jobs / Sec)    |
|  * Sidekiq (queue:<name>, retry)|                 |  * Wait Time vs Exec Duration |
+-------+-------------------------+                 +-----------------------+-------+
        |                                                                   |
        +---------------------------------+---------------------------------+
                                          |
                        +-----------------v-----------------+
                        |    Stall & Lock Leak Detector     |
                        |  * Worker Heartbeat Monitor       |
                        |  * Lock Expiration Tracker        |
                        |  * Zombie Active Job Alerts       |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |   Dead-Letter Queue (DLQ) Studio  |
                        |  * Error Signature Clustering     |
                        |  * Payload Diff & In-Place Edit   |
                        |  * 1-Click Mass Retry Engine      |
                        +-----------------+-----------------+
                                          |
                        +-----------------v-----------------+
                        |   Interactive Web Studio & CLI    |
                        |  * Real-Time Stream (Port 9190)   |
                        |  * Queue Latency Gauges           |
                        |  * Job Payload JSON Inspector     |
                        +-----------------+-----------------+
```

---

## 2. Directory Layout

```
queuelens/
├── cmd/
│   └── queuelens/
│       └── main.go               # CLI entrypoint (serve, stats, dlq, demo)
├── internal/
│   ├── client/
│   │   ├── redis_pool.go         # Redis/Valkey client with reconnect & cluster support
│   │   └── scanner.go            # SCAN keyspace iterator with cursor management
│   ├── adapters/
│   │   ├── adapter.go            # Common QueueAdapter interface & models
│   │   ├── bullmq.go             # BullMQ and Bull v3/v4 parser & mutator
│   │   ├── celery.go             # Python Celery parser & result unpacker
│   │   ├── asynq.go              # Go Asynq zset & hash parser
│   │   └── sidekiq.go            # Ruby Sidekiq queue & dead set parser
│   ├── engine/
│   │   ├── state_machine.go      # Normalized job lifecycle transitions
│   │   ├── stall_detector.go     # Lock TTL auditor & dead worker detector
│   │   └── metrics.go            # Sliding-window jobs/sec & latency estimator
│   ├── dlq/
│   │   ├── cluster.go            # Error signature normalizer & clustering engine
│   │   └── replayer.go           # Throttled mass replay & payload editor
│   ├── demo/
│   │   └── generator.go          # Embedded miniredis synthetic workload simulator
│   └── server/
│       ├── api.go                # REST API and WebSocket broadcaster
│       └── static.go             # Embedded React 19 web assets
├── web/
│   ├── src/
│   │   ├── components/
│   │   │   ├── Header.tsx        # Connection status & global controls
│   │   │   ├── QueueOverview.tsx # Metrics grid & queue breakdown cards
│   │   │   ├── JobStream.tsx     # Real-time virtualized job state stream
│   │   │   ├── DlqStudio.tsx     # Clustered dead-letter queue manager
│   │   │   ├── StallMonitor.tsx  # Worker heartbeats & locked zombie tasks
│   │   │   ├── PayloadModal.tsx  # JSON payload viewer, diff & editor
│   │   │   └── ConnectModal.tsx  # Redis host/port/auth configuration
│   │   ├── hooks/
│   │   │   └── useQueueStream.ts # WebSocket subscription hook
│   │   ├── App.tsx
│   │   ├── main.tsx
│   │   └── index.css
│   ├── package.json
│   ├── vite.config.ts
│   └── tsconfig.json
├── docs/
│   ├── images/                   # Screenshots and demo GIF
│   └── ARCHITECTURE.md
├── go.mod
├── go.sum
└── README.md
```

---

## 3. Implementation Steps

### Task 1: Go Workspace & Redis Client Pool
- Initialize Go module `github.com/alexandrmotologa/queuelens`.
- Implement `internal/client/redis_pool.go` wrapping `github.com/redis/go-redis/v9`.
- Support standalone Redis, Valkey, password authentication, and custom DB indexing.
- Implement cursor-based non-blocking `SCAN` in `internal/client/scanner.go`.

### Task 2: Multi-Queue Adapters
- `internal/adapters/adapter.go`:
  - Define `QueueType`: BullMQ, Celery, Asynq, Sidekiq.
  - Define `JobState`: `WAITING`, `ACTIVE`, `COMPLETED`, `FAILED`, `DELAYED`.
  - Define `QueueAdapter` interface with discovery, listing, inspection, retry, and purge capabilities.
- `internal/adapters/bullmq.go`:
  - Support `bull:<queue>:wait`, `active`, `completed`, `failed`, `delayed`.
  - Read job hashes, inspect locks (`bull:<queue>:<id>:lock`), parse stream events (`bull:<queue>:events`).
- `internal/adapters/celery.go`:
  - Detect Celery queue lists, parse task metadata `celery-task-meta-<id>`, extract status and traceback.
- `internal/adapters/asynq.go`:
  - Parse `asynq:{<queue>}:pending`, `active`, `scheduled`, `retry`, `archived`, `completed`.
  - Inspect task hashes `asynq:{<queue>}:t:<id>` and server heartbeats.
- `internal/adapters/sidekiq.go`:
  - Parse `queue:<name>`, `retry`, `schedule`, `dead`, and `processes` heartbeats.

### Task 3: State Machine, Metrics & Stall Detection
- `internal/engine/state_machine.go`:
  - Track transition timestamps and compute wait latency ($t_{pickup} - t_{enqueued}$) and execution duration.
- `internal/engine/metrics.go`:
  - Sliding-window calculator with 1-second buckets over 60 seconds (jobs/sec, failures/sec, latency p95).
- `internal/engine/stall_detector.go`:
  - Check active job lock keys and TTLs.
  - Detect workers missing heartbeats for longer than 30 seconds while holding active task locks.
  - Provide force-release mechanism for orphaned locks.

### Task 4: DLQ Studio & Error Clustering
- `internal/dlq/cluster.go`:
  - Normalize error messages (remove memory addresses, timestamps, unique IDs, numbers).
  - Compute cluster hashes to group dead jobs into root causes.
  - Return aggregated counts, sample payloads, and stack traces per cluster.
- `internal/dlq/replayer.go`:
  - Support re-queuing individual jobs or entire clusters.
  - Allow in-place payload JSON mutation before re-enqueue.
  - Token-bucket rate limiter during replay to prevent downstream overload.

### Task 5: Built-in Demo Engine
- `internal/demo/generator.go`:
  - Use `github.com/alicebob/miniredis/v2` to launch an in-process Redis instance.
  - Pre-populate realistic queues across BullMQ (e.g. `image-processing`, `email-notifications`), Celery (e.g. `video-transcode`), Asynq (e.g. `report-generation`), and Sidekiq (e.g. `webhook-delivery`).
  - Run background worker simulator that periodically enqueues jobs, moves some to active, completes some, fails some with realistic exceptions, and simulates worker lock stalls.
  - Enabled via `--demo` flag for instant zero-dependency evaluation.

### Task 6: REST API & WebSocket Event Broadcaster
- `internal/server/api.go`:
  - `/api/status`: Health and Redis connection information.
  - `/api/queues`: List all detected queues with counts per state.
  - `/api/queues/:type/:name/jobs`: List jobs filtered by state.
  - `/api/jobs/:type/:name/:id`: Get full job detail with payload and trace.
  - `/api/jobs/:type/:name/:id/retry`: Retry single job with optional payload edit.
  - `/api/jobs/:type/:name/:id`: Delete job.
  - `/api/dlq/clusters`: Get grouped DLQ clusters.
  - `/api/dlq/replay`: Mass replay a cluster or batch of IDs with rate limit.
  - `/api/stalls`: List stalled jobs and dead worker locks.
  - `/api/stalls/unlock`: Release orphaned lock.
  - `/api/metrics`: Live throughput and latency series.
  - `/api/ws`: WebSocket feed broadcasting real-time events.

### Task 7: CLI Commands
- `cmd/queuelens/main.go`:
  - `queuelens serve`: Start API and web studio (`--redis-url`, `--port`, `--demo`).
  - `queuelens stats`: Terminal summary table of active queues and throughput.
  - `queuelens dlq list`: Terminal list of failed jobs and error clusters.
  - `queuelens dlq replay`: Headless retry of failed jobs from CLI.

### Task 8: Web Studio (React 19 + Tailwind CSS)
- Build responsive, dark-mode cockpit in `web/`:
  - Summary metrics banner: Total queues, throughput (jobs/sec), active locks, DLQ count.
  - Interactive throughput and error rate line charts.
  - Queue cards with state distribution progress bars.
  - Real-time job stream with filtering by state and queue.
  - DLQ Studio tab with cluster cards, error trace modal, JSON editor, and 1-click mass replay.
  - Stall Monitor tab with worker lock countdowns and force unlock buttons.
- Compile static assets and embed into Go binary via `//go:embed`.

### Task 9: Verification & Testing
- Unit tests for all adapters with `miniredis`.
- Integration tests for clustering, state transitions, stall detection, and replayer.
- End-to-end API verification.
- Capture real UI screenshots and record demo GIF.
- Finalize portfolio synchronization and project assets.
