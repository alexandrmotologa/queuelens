# QueueLens Architecture & Internals

This document details the internal design of QueueLens, wire format parsers across Redis task queue systems, error clustering mechanisms, and stall detection algorithms.

---

## 1. High-Level System Architecture

QueueLens acts as a non-intrusive observability layer for background workers. It reads existing Redis data structures directly rather than requiring custom worker middleware or telemetry sidecars.

```
+-------------------------------------------------------------------------+
|                               Redis / Valkey                            |
|  * BullMQ (bull:*:*)              * Asynq (asynq:{*}:*)                 |
|  * Celery (celery, task-meta-*)   * Sidekiq (queue:*, dead, retry)      |
+------------------------------------+------------------------------------+
                                     |
                                     v Non-Blocking SCAN & Pipelined Reads
+-------------------------------------------------------------------------+
|                          QueueLens Backend Engine                       |
|                                                                         |
|  +--------------------+   +--------------------+   +-----------------+  |
|  | Multi-Queue        |   | State Machine      |   | Stall Auditor   |  |
|  | Adapters Registry  |-->| & Metrics Engine   |-->| & Lock Tracker  |  |
|  +--------------------+   +--------------------+   +-----------------+  |
|            |                                                |           |
|            v                                                v           |
|  +--------------------+                            +-----------------+  |
|  | DLQ Normalizer     |                            | Replay Engine   |  |
|  | & Cluster Hasher   |                            | (Token-Bucket)  |  |
|  +--------------------+                            +-----------------+  |
+------------------------------------+------------------------------------+
                                     |
                                     v HTTP REST & WebSocket (Port 9190)
+-------------------------------------------------------------------------+
|                     Embedded React 19 Web Studio                        |
|  * Throughput Gauges (Recharts)     * Real-Time Stream Table            |
|  * DLQ Signature Accordion          * In-Place Payload Editor & Replay  |
+-------------------------------------------------------------------------+
```

---

## 2. Multi-Queue Wire Formats

### BullMQ and Bull (TypeScript / Node.js)
BullMQ structures queues under a shared prefix (`bull:<queue_name>:`):
- `bull:<q>:wait`: List or Stream of job IDs awaiting execution.
- `bull:<q>:active`: Sorted Set or List of jobs currently being processed by workers.
- `bull:<q>:completed`: Sorted Set of successful job IDs scored by completion timestamp.
- `bull:<q>:failed`: Sorted Set of failed job IDs scored by failure timestamp.
- `bull:<q>:delayed`: Sorted Set of delayed jobs scored by target execution timestamp.
- `bull:<q>:<id>`: Hash containing task payload (`data`), options (`opts`), execution timestamps (`timestamp`, `processedOn`, `finishedOn`), retry counts (`attemptsMade`), and error stack traces (`failedReason`, `stacktrace`).
- `bull:<q>:<id>:lock`: String key created by the executing worker holding a unique lock token with a time-to-live (TTL).

### Celery (Python)
Celery utilizes standard Redis lists and key-value storage:
- `<queue_name>`: Redis List holding serialized task messages (AMQP/Celery protocol v1 or v2 format).
- `celery-task-meta-<task_id>`: Result backend key storing execution state (`SUCCESS`, `FAILURE`, `STARTED`, `RETRY`), result or error object, traceback, and completion timestamp.

### Asynq (Go)
Asynq isolates queue keys using Redis hash tags for cluster compatibility:
- `asynq:{<q>}:pending`: List or Sorted Set of pending task IDs.
- `asynq:{<q>}:active`: Sorted Set of active tasks scored by deadline timestamp.
- `asynq:{<q>}:completed`: Sorted Set of completed tasks scored by completion time.
- `asynq:{<q>}:archived`: Sorted Set representing the Dead-Letter Queue (DLQ).
- `asynq:{<q>}:retry` and `asynq:{<q>}:scheduled`: Sorted Sets holding retries and scheduled tasks.
- `asynq:{<q>}:t:<task_id>`: Hash containing serialized task definition (`msg`), current state, and retry count.
- `asynq:{<q>}:lease:<task_id>`: String key with TTL representing the active worker lock.

### Sidekiq (Ruby)
Sidekiq stores tasks as JSON blobs:
- `queues`: Redis Set containing the names of all registered queues.
- `queue:<name>`: Redis List holding queued job JSON definitions.
- `retry`: Sorted Set of retryable failed jobs scored by next retry timestamp.
- `schedule`: Sorted Set of scheduled jobs scored by execution timestamp.
- `dead`: Sorted Set containing dead-letter jobs exceeding max retries.
- `processes`: Set of worker process heartbeats.

---

## 3. Worker Stall and Lock Leak Detection

A worker stall occurs when a process crashes, is killed by an out-of-memory (OOM) killer, or network-partitions while actively executing a task.

The QueueLens Stall Auditor performs the following checks:
1. Gathers all job IDs currently marked in `active` state within the target queue.
2. Reads the corresponding lock key (e.g. `bull:<q>:<id>:lock` or `asynq:{<q>}:lease:<id>`).
3. Queries key TTL using Redis `PTTL`.
4. If the lock key is missing, expired (TTL <= 0), or significantly desynchronized from the job's elapsed execution time, QueueLens flags the task as `STALLED_WORKER`.
5. The operator can inspect the payload and invoke `ReleaseLock` to clear the orphaned lock and restore queue flow.

---

## 4. DLQ Error Signature Clustering

When downstream outages occur (such as third-party API rate limits or database connectivity drops), thousands of background tasks can fail within minutes. Browsing individual errors is inefficient because every task contains distinct parameters (user IDs, timestamps, invoice numbers).

QueueLens uses a deterministic clustering algorithm:

1. **Sanitization**: Strips non-deterministic tokens using compiled regular expressions:
   - UUIDs: `[0-9a-f]{8}-[0-9a-f]{4}-...` -> `<UUID>`
   - Host and IP addresses: `\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}(:\d+)?` -> `<HOST:PORT>`
   - Timestamps: `\d{4}-\d{2}-\d{2}[T\s]\d{2}:\d{2}:\d{2}...` -> `<TIMESTAMP>`
   - Identifiers: `[a-zA-Z_-]+_\d+` -> `<ID>`
   - Multi-digit numbers: `\b\d{2,}\b` -> `<NUM>`
2. **Fingerprinting**: Generates a 12-character SHA-256 hash of the normalized error signature string.
3. **Aggregation**: Groups failed jobs by hash, calculating job count, affected queues, first seen timestamp, last seen timestamp, and preserving a representative stack trace and payload for inspection.

---

## 5. Rate-Limited Mass Replay

Directly retrying 5,000 failed jobs simultaneously can trigger a thundering herd problem that knocks down downstream systems.

The QueueLens Replayer implements a token-bucket rate limiter:
- The operator configures a replay throughput limit (default 50 jobs/sec, range 5 to 250 jobs/sec).
- Jobs are re-enqueued sequentially with computed inter-request delays.
- Optional payload mutation allows correcting schema mismatches or broken parameters prior to re-enqueue.
