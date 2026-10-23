import React, { useState } from 'react';
import { ErrorCluster } from '../types';
import { AlertOctagon, RotateCw, ChevronDown, ChevronRight, Sliders, Layers, Code, CheckCircle, AlertCircle } from 'lucide-react';

interface DlqClusterViewProps {
  clusters: ErrorCluster[];
  onReplayCluster: (cluster: ErrorCluster, rateLimit: number) => Promise<void>;
  onInspectJob: (queueType: string, queue: string, jobId: string) => void;
  replayingClusterId: string | null;
}

export const DlqClusterView: React.FC<DlqClusterViewProps> = ({
  clusters,
  onReplayCluster,
  onInspectJob,
  replayingClusterId,
}) => {
  const [expandedId, setExpandedId] = useState<string | null>(clusters[0]?.id || null);
  const [rateLimit, setRateLimit] = useState<number>(50);

  const toggleExpand = (id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  };

  return (
    <div className="space-y-4">
      {/* Top Banner / Remediation Settings */}
      <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800 flex flex-col md:flex-row items-center justify-between gap-4">
        <div>
          <div className="flex items-center gap-2">
            <AlertOctagon className="h-5 w-5 text-red-400" />
            <h2 className="text-base font-semibold text-white">Dead-Letter Queue Remediation Studio</h2>
          </div>
          <p className="text-xs text-slate-400 mt-0.5">
            Jobs clustered by normalized root-cause exception signature. Replay clusters safely with rate-limiting.
          </p>
        </div>

        <div className="flex items-center gap-4 bg-slate-850 px-4 py-2 rounded-lg bg-slate-800/60 border border-slate-700/60">
          <div className="flex items-center gap-2 text-xs text-slate-300">
            <Sliders className="h-3.5 w-3.5 text-sky-400" />
            <span>Replay Rate Limit:</span>
            <span className="font-mono font-bold text-sky-400">{rateLimit} jobs/sec</span>
          </div>
          <input
            type="range"
            min="5"
            max="250"
            step="5"
            value={rateLimit}
            onChange={(e) => setRateLimit(Number(e.target.value))}
            className="w-28 accent-sky-500 cursor-pointer"
          />
        </div>
      </div>

      {clusters.length === 0 ? (
        <div className="p-16 text-center rounded-xl bg-slate-900/40 border border-slate-800">
          <CheckCircle className="h-10 w-10 text-emerald-500 mx-auto mb-3" />
          <h3 className="text-sm font-medium text-slate-200">Zero Failed Jobs Detected</h3>
          <p className="text-xs text-slate-500 mt-1 max-w-sm mx-auto">
            All background queues are healthy. No dead-letter queue records require remediation.
          </p>
        </div>
      ) : (
        <div className="space-y-3">
          {clusters.map((cluster) => {
            const isExpanded = expandedId === cluster.id;
            const isReplaying = replayingClusterId === cluster.id;

            return (
              <div
                key={cluster.id}
                className="rounded-xl border border-slate-800 bg-slate-900/70 overflow-hidden transition-all shadow-sm"
              >
                {/* Cluster Accordion Header */}
                <div
                  onClick={() => toggleExpand(cluster.id)}
                  className="p-4 flex flex-col md:flex-row items-start md:items-center justify-between gap-3 cursor-pointer hover:bg-slate-800/40 transition-colors"
                >
                  <div className="flex items-start gap-3 flex-1 overflow-hidden">
                    <button className="text-slate-400 mt-0.5">
                      {isExpanded ? <ChevronDown className="h-4 w-4" /> : <ChevronRight className="h-4 w-4" />}
                    </button>
                    <div className="overflow-hidden">
                      <div className="flex items-center gap-2 flex-wrap mb-1">
                        <span className="font-mono text-xs px-2 py-0.5 rounded bg-slate-800 border border-slate-700 text-slate-300">
                          #{cluster.id}
                        </span>
                        <span className="text-xs font-mono font-bold px-2 py-0.5 rounded-full bg-red-500/10 text-red-400 border border-red-500/20">
                          {cluster.count.toLocaleString()} Failed Jobs
                        </span>
                        <div className="flex items-center gap-1 text-[11px] text-slate-400">
                          <Layers className="h-3 w-3 text-slate-500" />
                          <span>Queues: {cluster.queues.join(', ')}</span>
                        </div>
                      </div>
                      <div className="font-mono text-xs text-slate-200 truncate" title={cluster.signature}>
                        {cluster.signature}
                      </div>
                    </div>
                  </div>

                  {/* 1-Click Mass Retry Button */}
                  <div className="flex items-center gap-2 w-full md:w-auto justify-end" onClick={(e) => e.stopPropagation()}>
                    <button
                      onClick={() => onInspectJob(cluster.sampleQueueType, cluster.sampleQueue, cluster.sampleJobId)}
                      className="px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
                    >
                      Inspect Payload
                    </button>
                    <button
                      disabled={isReplaying}
                      onClick={() => onReplayCluster(cluster, rateLimit)}
                      className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg text-xs font-semibold bg-sky-500 hover:bg-sky-400 text-white transition-all disabled:opacity-50 shadow-sm shadow-sky-500/20"
                    >
                      <RotateCw className={`h-3.5 w-3.5 ${isReplaying ? 'animate-spin' : ''}`} />
                      {isReplaying ? 'Replaying...' : `Retry All (${cluster.count})`}
                    </button>
                  </div>
                </div>

                {/* Expanded Details */}
                {isExpanded && (
                  <div className="border-t border-slate-800/80 p-4 bg-slate-950/40 space-y-4">
                    {/* Error and StackTrace */}
                    <div>
                      <div className="text-[11px] font-mono uppercase text-slate-400 mb-1.5 flex items-center gap-1.5">
                        <AlertCircle className="h-3.5 w-3.5 text-red-400" />
                        Sample Error Message
                      </div>
                      <div className="p-3 rounded-lg bg-slate-900 border border-slate-800 font-mono text-xs text-red-400 whitespace-pre-wrap">
                        {cluster.sampleError}
                      </div>
                    </div>

                    {cluster.sampleStackTrace && (
                      <div>
                        <div className="text-[11px] font-mono uppercase text-slate-400 mb-1.5 flex items-center gap-1.5">
                          <Code className="h-3.5 w-3.5 text-indigo-400" />
                          Stack Trace
                        </div>
                        <div className="p-3 rounded-lg bg-slate-900 border border-slate-800 font-mono text-[11px] text-slate-400 max-h-48 overflow-y-auto whitespace-pre-wrap">
                          {cluster.sampleStackTrace}
                        </div>
                      </div>
                    )}

                    {/* Metadata & Affected IDs */}
                    <div className="grid grid-cols-1 md:grid-cols-3 gap-3 text-xs">
                      <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/60">
                        <span className="text-slate-500">First Seen:</span>
                        <div className="font-mono text-slate-300 mt-0.5">
                          {new Date(cluster.firstSeen).toLocaleString()}
                        </div>
                      </div>
                      <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/60">
                        <span className="text-slate-500">Last Seen:</span>
                        <div className="font-mono text-slate-300 mt-0.5">
                          {new Date(cluster.lastSeen).toLocaleString()}
                        </div>
                      </div>
                      <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/60">
                        <span className="text-slate-500">Affected Job IDs:</span>
                        <div className="font-mono text-slate-300 mt-0.5 truncate" title={cluster.jobIds.join(', ')}>
                          {cluster.jobIds.slice(0, 5).join(', ')} {cluster.jobIds.length > 5 ? `+${cluster.jobIds.length - 5} more` : ''}
                        </div>
                      </div>
                    </div>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
};
