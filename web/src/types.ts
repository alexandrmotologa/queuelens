export type QueueType = 'bullmq' | 'celery' | 'asynq' | 'sidekiq';

export type JobState = 'waiting' | 'active' | 'completed' | 'failed' | 'delayed';

export interface QueueStats {
  waiting: number;
  active: number;
  completed: number;
  failed: number;
  delayed: number;
  total: number;
}

export interface QueueInfo {
  name: string;
  type: QueueType;
  stats: QueueStats;
  keyPrefix: string;
  workers: number;
  paused: boolean;
}

export interface Job {
  id: string;
  queue: string;
  queueType: QueueType;
  state: JobState;
  name: string;
  payload: Record<string, any>;
  rawPayload: string;
  errorReason?: string;
  stackTrace?: string;
  attempts: number;
  maxAttempts: number;
  enqueuedAt: string;
  startedAt?: string;
  finishedAt?: string;
  durationMs: number;
  waitTimeMs: number;
  lockKey?: string;
  lockTTLMs?: number;
  workerId?: string;
}

export interface ErrorCluster {
  id: string;
  signature: string;
  sampleError: string;
  sampleStackTrace: string;
  count: number;
  firstSeen: string;
  lastSeen: string;
  queues: string[];
  jobIds: string[];
  samplePayload: Record<string, any>;
  sampleJobId: string;
  sampleQueue: string;
  sampleQueueType: QueueType;
}

export interface StallWarning {
  jobId: string;
  queue: string;
  queueType: QueueType;
  lockKey: string;
  lockTTLMs: number;
  activeSince: string;
  elapsedMs: number;
  workerId: string;
  reason: string;
}

export interface MetricPoint {
  time: string;
  timestamp: number;
  throughput: number;
  completed: number;
  failed: number;
  enqueued: number;
  avgWaitMs: number;
  avgExecMs: number;
}

export interface MetricsSummary {
  jobsPerSecond: number;
  completedPerSec: number;
  failedPerSec: number;
  errorRatePercent: number;
  averageWaitMs: number;
  averageExecMs: number;
  totalActiveQueues: number;
  totalWaitingJobs: number;
  totalActiveJobs: number;
  totalFailedJobs: number;
  timeToEmptySeconds?: number;
  slaBreached?: boolean;
  slaMessage?: string;
  history: MetricPoint[];
}

export interface JobEvent {
  id: string;
  queue: string;
  queueType: QueueType;
  name: string;
  fromState: JobState | '';
  toState: JobState;
  timestamp: string;
  durationMs?: number;
  waitTimeMs?: number;
  errorReason?: string;
}

export interface ServerStatus {
  connected: boolean;
  redisAddr: string;
  isDemo: boolean;
  server: {
    version: string;
    mode: string;
    os: string;
    connectedClients: number;
    usedMemoryHuman: string;
    totalKeys: number;
    uptimeInSeconds: number;
  };
}
