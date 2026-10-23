# QueueLens

Universal background task queue observability studio for Redis and Valkey. Auto-detects BullMQ, Celery, Asynq, and Sidekiq queues, tracks job state transitions in real time, audits worker locks to detect stalls, and clusters dead-letter queue failures for 1-click remediation.

![QueueLens Live Studio Demo](docs/images/queuelens_demo.gif)

---

## Capabilities

- **Universal Auto-Discovery**: Identifies active queues across Node.js BullMQ, Python Celery, Go Asynq, and Ruby Sidekiq without requiring code changes or agents inside your worker containers.
- **Real-Time Job Lifecycle Visualizer**: Streams state transitions (`waiting` -> `active` -> `completed` / `failed` -> `delayed`) via WebSockets with zero page refreshes.
- **Worker Stall & Lock Leak Auditor**: Detects zombie active jobs holding Redis locks whose worker heartbeat expired or crashed mid-execution, providing a 1-click force unlock mechanism.
- **DLQ Studio & Error Signature Clustering**: Normalizes dynamic timestamps, UUIDs, hostnames, and identifiers from error traces to aggregate thousands of failures into actionable root causes.
- **1-Click Mass Replay**: Re-enqueues failed clusters with configurable token-bucket rate limiting (5 to 250 jobs/sec) to avoid cascading retry storms against downstream databases and APIs.
- **In-Place Payload Mutation**: Allows editing malformed job JSON payloads directly inside the UI before retrying.
- **Single Static Binary**: The React 19 web dashboard is compiled and embedded into the Go binary (`//go:embed`), requiring zero Node.js runtime on production servers.
- **Zero-Dependency Demo Mode**: Run with `--demo` to start an in-process Redis engine pre-loaded with synthetic multi-framework workloads.

---

## User Interface

### Queue Overview & Live Velocity
Visualizes queue depth, active worker concurrency, proportional state distributions, and rolling 60-second throughput (jobs/sec) alongside completion and error rates.

![QueueLens Overview](docs/images/queuelens-1.png)

### Real-Time Transition Stream
Virtualized event table monitoring state transitions, execution runtimes, and wait latencies across all discovered queues.

![QueueLens Live Stream](docs/images/queuelens-2.png)

### Dead-Letter Queue (DLQ) Remediation Studio
Groups failed jobs by normalized error signature, displays sample stack traces, and provides 1-click mass replay controls.

![QueueLens DLQ Studio](docs/images/queuelens-3.png)

### Worker Stall & Lock Monitor
Audits active job lease keys, compares worker heartbeats, and flags orphaned locks left behind by killed or OOM workers.

![QueueLens Stall Monitor](docs/images/queuelens-4.png)

---

## Quick Start

### Installation

#### Pre-built Binary
Download the pre-compiled binary for Linux, macOS, or Windows from the GitHub Releases page:

```bash
# Verify installation
queuelens version
```

#### Build from Source
Requirements: Go 1.23+ and Node.js 20+ (for compiling the frontend).

```bash
git clone https://github.com/alexandrmotologa/queuelens.git
cd queuelens

# Build frontend assets
cd web
npm install
npm run build
cd ..

# Compile single binary
go build -o queuelens ./cmd/queuelens
```

---

## Usage

### 1. Zero-Dependency Demo Mode
Evaluate QueueLens immediately without configuring a local Redis server:

```bash
./queuelens --demo
```
Access the web dashboard at `http://localhost:9190`.

### 2. Connect to Redis or Valkey
Point QueueLens to any standalone Redis instance, Valkey server, or managed cluster:

```bash
# Connect with default port 9190
./queuelens serve --redis-url "redis://:yourpassword@127.0.0.1:6379/0"

# Custom port
./queuelens serve --redis-url "redis://10.0.1.50:6379" --port 8080

# Using environment variables
export REDIS_URL="redis://127.0.0.1:6379/0"
export PORT="9190"
./queuelens serve
```

### 3. Command Line Diagnostics

#### Inspect Queues via CLI
Print an ASCII overview table of detected queues, job states, and worker counts:

```bash
./queuelens stats --redis-url "redis://127.0.0.1:6379/0"
```

Output:
```text
QUEUE                    TYPE       WAITING  ACTIVE   FAILED   DELAYED  TOTAL   
--------------------------------------------------------------------------------
email-notifications      bullmq     8        1        5        0        14      
image-processing         bullmq     6        0        0        0        6       
report-generation        asynq      4        0        3        0        7       
webhook-delivery         sidekiq    5        0        4        0        9       
--------------------------------------------------------------------------------
Total detected queues: 4
```

#### Review Dead-Letter Clusters
```bash
./queuelens dlq list --redis-url "redis://127.0.0.1:6379/0"
```

Output:
```text
CLUSTER ID   COUNT    QUEUES               ERROR SIGNATURE                       
--------------------------------------------------------------------------------
1927816237ef 5        email-notifications  ConnectionRefused: dial tcp <HOST:PORT>
6df1da24c59e 4        webhook-delivery     Stripe::RateLimitError: HTTP <NUM>... 
d6d5e7fafcfd 3        report-generation    pgx: connection pool exhausted...     
--------------------------------------------------------------------------------
Total dead jobs: 12 across 3 distinct error clusters
```

---

## Supported Task Queue Frameworks

| Framework | Language | Discovery Pattern | State Tracking | Stall / Lock Detection |
| :--- | :--- | :--- | :--- | :--- |
| **BullMQ & Bull** | TypeScript / Node.js | `bull:*:meta`, `bull:*:wait` | Lists, Hashes, Sorted Sets | `bull:<q>:<id>:lock` TTL check |
| **Celery** | Python | `celery`, `celery-task-meta-*` | Task Lists, Result Hashes | Active task metadata state |
| **Asynq** | Go | `asynq:queues`, `asynq:{*}:*` | Sorted Sets, Task Hashes | `asynq:{<q>}:lease:<id>` TTL check |
| **Sidekiq** | Ruby | `queues`, `queue:*`, `dead` | Lists, Sorted Sets (`dead`, `retry`) | Process heartbeat validation |

---

## Architecture

QueueLens utilizes non-blocking `SCAN` iteration to avoid locking Redis event loops. Read operations inspect metadata hashes and sorted sets, while write operations (such as DLQ retry or lock release) execute atomically via Redis pipelines.

For detailed wire formats, clustering algorithms, and state machine transitions, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## License

MIT License. Copyright (c) 2026 Alexandr Motologa.
