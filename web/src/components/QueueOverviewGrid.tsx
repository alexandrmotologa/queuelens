import React from 'react';
import { QueueInfo, MetricsSummary, QueueType } from '../types';
import { Zap, Clock, AlertTriangle, CheckCircle, Flame, Layers, Trash2, ArrowUpRight } from 'lucide-react';
import { ResponsiveContainer, AreaChart, Area, XAxis, YAxis, Tooltip } from 'recharts';

interface QueueOverviewGridProps {
  queues: QueueInfo[];
  metrics: MetricsSummary | null;
  onSelectQueue: (queueName: string, state?: string) => void;
  onPurgeQueue: (qType: QueueType, queue: string, state: string) => void;
  stallCount: number;
}

export const QueueOverviewGrid: React.FC<QueueOverviewGridProps> = ({
  queues,
  metrics,
  onSelectQueue,
  onPurgeQueue,
  stallCount,
}) => {
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

  const chartData = metrics?.history || [];

  return (
    <div className="space-y-6">
      {/* Top Metrics Banner */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-3.5">
        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5">
            <span>Throughput</span>
            <Zap className="h-4 w-4 text-sky-400" />
          </div>
          <div className="text-2xl font-bold font-mono text-white">
            {metrics ? metrics.jobsPerSecond.toFixed(1) : '0.0'}
            <span className="text-xs text-slate-400 font-sans ml-1">jobs/s</span>
          </div>
          <div className="text-[11px] text-slate-500 mt-1">Rolling 60s rate</div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5">
            <span>Error Rate</span>
            <AlertTriangle className="h-4 w-4 text-amber-400" />
          </div>
          <div className="text-2xl font-bold font-mono text-white">
            {metrics ? metrics.errorRatePercent.toFixed(1) : '0.0'}%
          </div>
          <div className="text-[11px] text-slate-500 mt-1">Failed vs processed</div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5">
            <span>Waiting Backlog</span>
            <Clock className="h-4 w-4 text-indigo-400" />
          </div>
          <div className="text-2xl font-bold font-mono text-white">
            {metrics ? metrics.totalWaitingJobs.toLocaleString() : '0'}
          </div>
          <div className="text-[11px] text-slate-500 mt-1">Across all queues</div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5">
            <span>DLQ / Failed</span>
            <Flame className="h-4 w-4 text-red-400" />
          </div>
          <div className="text-2xl font-bold font-mono text-white">
            {metrics ? metrics.totalFailedJobs.toLocaleString() : '0'}
          </div>
          <div className="text-[11px] text-slate-500 mt-1">Requires remediation</div>
        </div>

        <div className="p-4 rounded-xl bg-slate-900/60 border border-slate-800/80 shadow-sm">
          <div className="flex items-center justify-between text-slate-400 text-xs mb-1.5">
            <span>Worker Stalls</span>
            <AlertTriangle className="h-4 w-4 text-amber-500" />
          </div>
          <div className="text-2xl font-bold font-mono text-white">
            {stallCount}
          </div>
          <div className="text-[11px] text-slate-500 mt-1">Dead worker locks</div>
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
            <p className="text-xs text-slate-400">Auto-detected Redis task queues across frameworks</p>
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
                      </div>
                      <button
                        onClick={() => onSelectQueue(q.name)}
                        className="text-xs text-sky-400 hover:text-sky-300 flex items-center gap-1 font-medium transition-colors"
                      >
                        Inspect
                        <ArrowUpRight className="h-3 w-3" />
                      </button>
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
                        <div className="text-slate-400 text-[10px]">Waiting</div>
                        <div className="font-mono font-semibold text-indigo-400">{q.stats.waiting}</div>
                      </div>
                      <div>
                        <div className="text-slate-400 text-[10px]">Active</div>
                        <div className="font-mono font-semibold text-sky-400">{q.stats.active}</div>
                      </div>
                      <div>
                        <div className="text-slate-400 text-[10px]">Failed</div>
                        <div className="font-mono font-semibold text-red-400">{q.stats.failed}</div>
                      </div>
                      <div>
                        <div className="text-slate-400 text-[10px]">Completed</div>
                        <div className="font-mono font-semibold text-emerald-400">{q.stats.completed}</div>
                      </div>
                    </div>
                  </div>

                  {/* Actions footer */}
                  <div className="flex items-center justify-between pt-2 text-[11px] text-slate-500">
                    <span className="font-mono">{q.keyPrefix}</span>
                    {q.stats.failed > 0 && (
                      <button
                        onClick={() => onPurgeQueue(q.type, q.name, 'failed')}
                        className="text-red-400 hover:text-red-300 flex items-center gap-1 transition-colors"
                        title="Purge Failed Jobs"
                      >
                        <Trash2 className="h-3 w-3" />
                        Purge DLQ
                      </button>
                    )}
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
