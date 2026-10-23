import React, { useState } from 'react';
import { QueueInfo, MetricsSummary, QueueType } from '../types';
import { Zap, Clock, AlertTriangle, CheckCircle, Flame, Layers, Trash2, ArrowUpRight, Pause, Play, Hourglass, ShieldAlert } from 'lucide-react';
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip } from 'recharts';

interface QueueOverviewGridProps {
  queues: QueueInfo[];
  metrics: MetricsSummary | null;
  onSelectQueue: (queueName: string, state?: string) => void;
  onPurgeQueue: (qType: QueueType, queue: string, state: string) => void;
  onTogglePauseQueue: (qType: QueueType, queue: string, currentlyPaused: boolean) => Promise<void>;
  onDrainQueue: (qType: QueueType, queue: string) => Promise<void>;
  stallCount: number;
}

export const QueueOverviewGrid: React.FC<QueueOverviewGridProps> = ({
  queues,
  metrics,
  onSelectQueue,
  onPurgeQueue,
  onTogglePauseQueue,
  onDrainQueue,
  stallCount,
}) => {
  const [loadingAction, setLoadingAction] = useState<string | null>(null);

  const getBadgeColor = (type: QueueType) => {
    switch (type) {
      case 'bullmq':
        return 'bg-amber-500/10 text-amber-400 border-amber-500/30';
      case 'celery':
        return 'bg-emerald-500/10 text-emerald-400 border-emerald-500/30';
      case 'asynq':
        return 'bg-cyan-500/10 text-cyan-400 border-cyan-500/30';
      case 'sidekiq':
        return 'bg-rose-500/10 text-rose-400 border-rose-500/30';
      default:
        return 'bg-slate-500/10 text-slate-400 border-slate-500/30';
    }
  };

  const formatTTE = (secs?: number) => {
    if (secs === undefined || secs <= 0) return 'Instant';
    if (secs < 60) return `${secs}s`;
    const m = Math.floor(secs / 60);
    const s = secs % 60;
    if (m < 60) return `~${m}m ${s}s`;
    const h = Math.floor(m / 60);
    return `~${h}h ${m % 60}m`;
  };

  const chartData = metrics?.history || [];

  return (
    <div className="space-y-6">
      {/* SLA Breach Warning Banner */}
      {metrics?.slaBreached && (
        <div className="p-4 rounded-xl bg-red-950/40 border border-red-800/60 flex items-center justify-between text-red-300 text-xs shadow-md animate-pulse">
          <div className="flex items-center gap-2.5">
            <ShieldAlert className="h-5 w-5 text-red-400 flex-shrink-0" />
            <div>
              <span className="font-semibold text-red-200">SLA Degradation Warning: </span>
              {metrics.slaMessage || 'Queue wait times or error rates are currently breaching operational targets'}
            </div>
          </div>
          <span className="font-mono text-[11px] bg-red-900/60 px-2.5 py-1 rounded border border-red-700/60">
            Action Recommended
          </span>
        </div>
      )}

      {/* Top Metrics Banner */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-5 gap-3 sm:gap-3.5">
        <div className="p-3.5 sm:p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5 whitespace-nowrap">
            <span>Throughput</span>
            <Zap className="h-4 w-4 text-sky-400 shrink-0" />
          </div>
          <div className="text-xl sm:text-2xl font-bold font-mono text-white whitespace-nowrap">
            {metrics ? metrics.jobsPerSecond.toFixed(1) : '0.0'}
            <span className="text-xs text-slate-400 font-sans ml-1">j/s</span>
          </div>
          <div className="text-[10px] sm:text-[11px] text-slate-500 mt-1 whitespace-nowrap">Rolling 60s velocity</div>
        </div>

        <div className="p-3.5 sm:p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5 whitespace-nowrap">
            <span>Time-to-Empty</span>
            <Hourglass className="h-4 w-4 text-indigo-400 shrink-0" />
          </div>
          <div className="text-xl sm:text-2xl font-bold font-mono text-white whitespace-nowrap truncate">
            {formatTTE(metrics?.timeToEmptySeconds)}
          </div>
          <div className="text-[10px] sm:text-[11px] text-slate-500 mt-1 whitespace-nowrap">Backlog drain pace</div>
        </div>

        <div className="p-3.5 sm:p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5 whitespace-nowrap">
            <span>Error Rate</span>
            <AlertTriangle className="h-4 w-4 text-amber-400 shrink-0" />
          </div>
          <div className="text-xl sm:text-2xl font-bold font-mono text-white whitespace-nowrap">
            {metrics ? metrics.errorRatePercent.toFixed(1) : '0.0'}%
          </div>
          <div className="text-[10px] sm:text-[11px] text-slate-500 mt-1 whitespace-nowrap">Failed vs completed</div>
        </div>

        <div className="p-3.5 sm:p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5 whitespace-nowrap">
            <span>DLQ / Failed</span>
            <Flame className="h-4 w-4 text-red-400 shrink-0" />
          </div>
          <div className="text-xl sm:text-2xl font-bold font-mono text-white whitespace-nowrap">
            {metrics ? metrics.totalFailedJobs.toLocaleString() : '0'}
          </div>
          <div className="text-[10px] sm:text-[11px] text-slate-500 mt-1 whitespace-nowrap">Dead-letter count</div>
        </div>

        {/* 5th Card spans 2 columns on 2-col mobile for balanced grid */}
        <div className="col-span-2 sm:col-span-1 p-3.5 sm:p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm flex flex-col justify-between">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5 whitespace-nowrap">
            <span>Worker Stalls</span>
            <ShieldAlert className="h-4 w-4 text-amber-500 shrink-0" />
          </div>
          <div className="text-xl sm:text-2xl font-bold font-mono text-white whitespace-nowrap">
            {stallCount}
          </div>
          <div className="text-[10px] sm:text-[11px] text-slate-500 mt-1 whitespace-nowrap">Dead worker locks</div>
        </div>
      </div>

      {/* Real-Time Throughput Chart */}
      <div className="p-5 rounded-xl bg-slate-900/60 border border-slate-800/80">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-sm font-semibold text-slate-200">Throughput Velocity & Task Completions</h3>
            <p className="text-xs text-slate-500">Live sliding window updates streamed every second</p>
          </div>
          <div className="flex items-center gap-4 text-xs font-mono">
            <span className="flex items-center gap-1.5 text-sky-400">
              <span className="h-2 w-2 rounded-full bg-sky-400" />
              Completed
            </span>
            <span className="flex items-center gap-1.5 text-red-400">
              <span className="h-2 w-2 rounded-full bg-red-400" />
              Failed
            </span>
            <span className="flex items-center gap-1.5 text-indigo-400">
              <span className="h-2 w-2 rounded-full bg-indigo-400" />
              Enqueued
            </span>
          </div>
        </div>

        <div className="h-48 w-full">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={chartData}>
              <defs>
                <linearGradient id="colorCompleted" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#38bdf8" stopOpacity={0.4}/>
                  <stop offset="95%" stopColor="#38bdf8" stopOpacity={0}/>
                </linearGradient>
                <linearGradient id="colorFailed" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#ef4444" stopOpacity={0.4}/>
                  <stop offset="95%" stopColor="#ef4444" stopOpacity={0}/>
                </linearGradient>
              </defs>
              <XAxis dataKey="time" stroke="#475569" fontSize={10} tickLine={false} />
              <YAxis stroke="#475569" fontSize={10} tickLine={false} />
              <Tooltip
                contentStyle={{ backgroundColor: '#0f172a', borderColor: '#334155', borderRadius: '8px', fontSize: '12px' }}
                labelStyle={{ color: '#94a3b8' }}
              />
              <Area type="monotone" dataKey="completed" stroke="#38bdf8" fillOpacity={1} fill="url(#colorCompleted)" strokeWidth={2} />
              <Area type="monotone" dataKey="failed" stroke="#ef4444" fillOpacity={1} fill="url(#colorFailed)" strokeWidth={2} />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      </div>

      {/* Discovered Queues Section */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-base font-semibold text-white">Discovered Queues ({queues.length})</h2>
            <p className="text-xs text-slate-400">Control queue lifecycle, pause consumption, and purge backlogs</p>
          </div>
        </div>

        {queues.length === 0 ? (
          <div className="p-12 text-center rounded-xl bg-slate-900/40 border border-slate-800">
            <Layers className="h-10 w-10 text-slate-600 mx-auto mb-3" />
            <h3 className="text-sm font-medium text-slate-300">No active queues discovered</h3>
            <p className="text-xs text-slate-500 mt-1 max-w-sm mx-auto">
              Ensure workers or job producers are connected to Redis. Run with --demo to evaluate with pre-seeded data.
            </p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
            {queues.map((q) => {
              const total = Math.max(q.stats.total, 1);
              const pWait = ((q.stats.waiting / total) * 100).toFixed(0);
              const pActive = ((q.stats.active / total) * 100).toFixed(0);
              const pFail = ((q.stats.failed / total) * 100).toFixed(0);
              const pDone = ((q.stats.completed / total) * 100).toFixed(0);
              const isActionLoading = loadingAction === q.name;

              return (
                <div
                  key={`${q.type}-${q.name}`}
                  className="p-5 rounded-xl bg-slate-900/70 border border-slate-800/80 hover:border-slate-700 transition-all hover:shadow-lg flex flex-col justify-between"
                >
                  <div>
                    {/* Header */}
                    <div className="flex items-center justify-between gap-2 mb-3">
                      <div className="flex items-center gap-2 overflow-hidden">
                        <span className={`text-[10px] uppercase font-mono font-bold px-2 py-0.5 rounded border ${getBadgeColor(q.type)}`}>
                          {q.type}
                        </span>
                        <h3 className="font-semibold text-slate-100 text-sm truncate" title={q.name}>
                          {q.name}
                        </h3>
                        {q.paused && (
                          <span className="text-[10px] font-mono px-2 py-0.5 rounded bg-amber-500/20 text-amber-400 border border-amber-500/30">
                            PAUSED
                          </span>
                        )}
                      </div>

                      <div className="flex items-center gap-1.5">
                        {/* Pause / Resume Button */}
                        <button
                          disabled={isActionLoading}
                          onClick={async () => {
                            setLoadingAction(q.name);
                            try {
                              await onTogglePauseQueue(q.type, q.name, q.paused);
                            } finally {
                              setLoadingAction(null);
                            }
                          }}
                          title={q.paused ? 'Resume Queue Consumption' : 'Pause Queue Consumption'}
                          className={`p-1.5 rounded-lg border text-xs transition-all ${
                            q.paused
                              ? 'bg-emerald-500/20 hover:bg-emerald-500/30 border-emerald-500/40 text-emerald-400'
                              : 'bg-slate-800 hover:bg-slate-700 border-slate-700 text-slate-300'
                          }`}
                        >
                          {q.paused ? <Play className="h-3.5 w-3.5 fill-current" /> : <Pause className="h-3.5 w-3.5" />}
                        </button>

                        <button
                          onClick={() => onSelectQueue(q.name)}
                          className="text-xs text-sky-400 hover:text-sky-300 inline-flex items-center gap-1 font-medium transition-colors whitespace-nowrap ml-1"
                        >
                          Inspect
                          <ArrowUpRight className="h-3 w-3 shrink-0" />
                        </button>
                      </div>
                    </div>

                    {/* Proportional State Bar */}
                    <div className="h-2 w-full rounded-full bg-slate-800 flex overflow-hidden my-3">
                      <div style={{ width: `${pWait}%` }} className="bg-indigo-500" title={`Waiting: ${q.stats.waiting}`} />
                      <div style={{ width: `${pActive}%` }} className="bg-sky-400" title={`Active: ${q.stats.active}`} />
                      <div style={{ width: `${pFail}%` }} className="bg-red-500" title={`Failed: ${q.stats.failed}`} />
                      <div style={{ width: `${pDone}%` }} className="bg-emerald-500" title={`Completed: ${q.stats.completed}`} />
                    </div>

                    {/* Stats Breakdown */}
                    <div className="grid grid-cols-4 gap-2 text-center text-xs py-2 border-y border-slate-800/60 my-3">
                      <div>
                        <div className="text-slate-400 text-[10px] whitespace-nowrap">Waiting</div>
                        <div className="font-mono font-semibold text-indigo-400 whitespace-nowrap">{q.stats.waiting}</div>
                      </div>
                      <div>
                        <div className="text-slate-400 text-[10px] whitespace-nowrap">Active</div>
                        <div className="font-mono font-semibold text-sky-400 whitespace-nowrap">{q.stats.active}</div>
                      </div>
                      <div>
                        <div className="text-slate-400 text-[10px] whitespace-nowrap">Failed</div>
                        <div className="font-mono font-semibold text-red-400 whitespace-nowrap">{q.stats.failed}</div>
                      </div>
                      <div>
                        <div className="text-slate-400 text-[10px] whitespace-nowrap">Completed</div>
                        <div className="font-mono font-semibold text-emerald-400 whitespace-nowrap">{q.stats.completed}</div>
                      </div>
                    </div>
                  </div>

                  {/* Actions footer */}
                  <div className="flex items-center justify-between pt-2 text-[11px] text-slate-500">
                    <span className="font-mono truncate mr-2">{q.keyPrefix}</span>
                    <div className="flex items-center gap-3 shrink-0">
                      {q.stats.waiting > 0 && (
                        <button
                          onClick={() => onDrainQueue(q.type, q.name)}
                          className="text-slate-400 hover:text-slate-200 transition-colors whitespace-nowrap"
                          title="Drain waiting jobs"
                        >
                          Drain
                        </button>
                      )}
                      {q.stats.failed > 0 && (
                        <button
                          onClick={() => onPurgeQueue(q.type, q.name, 'failed')}
                          className="text-red-400 hover:text-red-300 inline-flex items-center gap-1 transition-colors whitespace-nowrap"
                          title="Purge Failed Jobs"
                        >
                          <Trash2 className="h-3 w-3 shrink-0" />
                          <span>Purge DLQ</span>
                        </button>
                      )}
                    </div>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>
    </div>
  );
};
