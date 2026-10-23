import React, { useState } from 'react';
import { ErrorCluster } from '../types';
import { AlertOctagon, RotateCw, ChevronDown, ChevronRight, Sliders, Layers, Code, CheckCircle, AlertCircle, Download, Upload, Replace, X, Check } from 'lucide-react';

interface DlqClusterViewProps {
  clusters: ErrorCluster[];
  onReplayCluster: (cluster: ErrorCluster, rateLimit: number) => Promise<void>;
  onInspectJob: (queueType: string, queue: string, jobId: string) => void;
  onImportBundle: (bundleData: string) => Promise<void>;
  onBatchTransform: (cluster: ErrorCluster, findText: string, replaceText: string, rateLimit: number) => Promise<void>;
  replayingClusterId: string | null;
}

export const DlqClusterView: React.FC<DlqClusterViewProps> = ({
  clusters,
  onReplayCluster,
  onInspectJob,
  onImportBundle,
  onBatchTransform,
  replayingClusterId,
}) => {
  const [expandedId, setExpandedId] = useState<string | null>(clusters[0]?.id || null);
  const [rateLimit, setRateLimit] = useState<number>(50);

  // Import Modal State
  const [isImportOpen, setIsImportOpen] = useState(false);
  const [importText, setImportText] = useState('');
  const [importing, setImporting] = useState(false);

  // Batch Transform Modal State
  const [transformCluster, setTransformCluster] = useState<ErrorCluster | null>(null);
  const [findText, setFindText] = useState('');
  const [replaceText, setReplaceText] = useState('');
  const [transforming, setTransforming] = useState(false);

  const toggleExpand = (id: string) => {
    setExpandedId((prev) => (prev === id ? null : id));
  };

  const handleExport = (format: 'json' | 'csv', clusterId?: string) => {
    const params = new URLSearchParams({ format });
    if (clusterId) params.append('clusterId', clusterId);
    window.open(`/api/dlq/export?${params.toString()}`, '_blank');
  };

  const handleImportSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!importText.trim()) return;
    setImporting(true);
    try {
      await onImportBundle(importText);
      setIsImportOpen(false);
      setImportText('');
    } finally {
      setImporting(false);
    }
  };

  const handleTransformSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!transformCluster || !findText) return;
    setTransforming(true);
    try {
      await onBatchTransform(transformCluster, findText, replaceText, rateLimit);
      setTransformCluster(null);
      setFindText('');
      setReplaceText('');
    } finally {
      setTransforming(false);
    }
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
            Jobs clustered by normalized root-cause exception signature. Export, import bundles, or replay safely.
          </p>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          {/* Export Controls */}
          <div className="flex items-center gap-1.5 bg-slate-800/80 p-1 rounded-lg border border-slate-700">
            <button
              onClick={() => handleExport('json')}
              className="flex items-center gap-1 px-2.5 py-1 rounded text-xs text-slate-300 hover:text-white hover:bg-slate-700 transition-colors"
              title="Export all failed jobs as JSON bundle"
            >
              <Download className="h-3 w-3" />
              JSON
            </button>
            <button
              onClick={() => handleExport('csv')}
              className="flex items-center gap-1 px-2.5 py-1 rounded text-xs text-slate-300 hover:text-white hover:bg-slate-700 transition-colors"
              title="Export all failed jobs as CSV"
            >
              <Download className="h-3 w-3" />
              CSV
            </button>
          </div>

          {/* Import Button */}
          <button
            onClick={() => setIsImportOpen(true)}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs font-medium bg-slate-800 hover:bg-slate-700 border border-slate-700 text-slate-200 transition-colors"
          >
            <Upload className="h-3.5 w-3.5" />
            Import Bundle
          </button>

          {/* Rate Limiter Slider */}
          <div className="flex items-center gap-3 bg-slate-800/60 px-3.5 py-1.5 rounded-lg border border-slate-700/60">
            <div className="flex items-center gap-1.5 text-xs text-slate-300">
              <Sliders className="h-3 w-3 text-sky-400" />
              <span className="font-mono text-sky-400 font-semibold">{rateLimit} j/s</span>
            </div>
            <input
              type="range"
              min="5"
              max="250"
              step="5"
              value={rateLimit}
              onChange={(e) => setRateLimit(Number(e.target.value))}
              className="w-20 accent-sky-500 cursor-pointer"
            />
          </div>
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

                  {/* Actions Header */}
                  <div className="flex items-center gap-2 w-full md:w-auto justify-end flex-wrap" onClick={(e) => e.stopPropagation()}>
                    <button
                      onClick={() => handleExport('json', cluster.id)}
                      className="p-1.5 rounded-lg bg-slate-800 hover:bg-slate-700 text-slate-400 hover:text-white transition-colors"
                      title="Export cluster to JSON"
                    >
                      <Download className="h-3.5 w-3.5" />
                    </button>
                    <button
                      onClick={() => setTransformCluster(cluster)}
                      className="flex items-center gap-1 px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
                      title="Find and replace text in payloads before replay"
                    >
                      <Replace className="h-3.5 w-3.5 text-indigo-400" />
                      Batch Transform
                    </button>
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

      {/* Import Bundle Modal */}
      {isImportOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div className="w-full max-w-xl rounded-2xl bg-slate-900 border border-slate-800 shadow-2xl p-5 space-y-4">
            <div className="flex items-center justify-between border-b border-slate-800 pb-3">
              <div className="flex items-center gap-2">
                <Upload className="h-5 w-5 text-sky-400" />
                <h3 className="font-semibold text-slate-100 text-sm">Import & Replay DLQ Bundle</h3>
              </div>
              <button onClick={() => setIsImportOpen(false)} className="text-slate-400 hover:text-slate-200">
                <X className="h-5 w-5" />
              </button>
            </div>
            <form onSubmit={handleImportSubmit} className="space-y-3">
              <div>
                <label className="text-xs text-slate-300 block mb-1">Paste DLQ Bundle JSON:</label>
                <textarea
                  rows={8}
                  value={importText}
                  onChange={(e) => setImportText(e.target.value)}
                  placeholder='{"version": "1.0", "jobs": [...]}'
                  className="w-full font-mono text-xs p-3 rounded-xl bg-slate-950 border border-slate-800 text-slate-200 focus:outline-none focus:border-sky-500"
                />
              </div>
              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setIsImportOpen(false)}
                  className="px-3 py-1.5 text-xs text-slate-400 hover:text-slate-200"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={importing || !importText.trim()}
                  className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-xs font-semibold bg-sky-500 hover:bg-sky-400 text-white transition-all disabled:opacity-50"
                >
                  <Check className="h-3.5 w-3.5" />
                  {importing ? 'Importing...' : 'Import & Re-Enqueue'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Batch Transform Modal */}
      {transformCluster && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
          <div className="w-full max-w-lg rounded-2xl bg-slate-900 border border-slate-800 shadow-2xl p-5 space-y-4">
            <div className="flex items-center justify-between border-b border-slate-800 pb-3">
              <div className="flex items-center gap-2">
                <Replace className="h-5 w-5 text-indigo-400" />
                <h3 className="font-semibold text-slate-100 text-sm">Batch Transform & Replay Cluster</h3>
              </div>
              <button onClick={() => setTransformCluster(null)} className="text-slate-400 hover:text-slate-200">
                <X className="h-5 w-5" />
              </button>
            </div>
            <p className="text-xs text-slate-400">
              Apply find-and-replace across all {transformCluster.count} jobs in cluster #{transformCluster.id} before re-enqueue.
            </p>
            <form onSubmit={handleTransformSubmit} className="space-y-3">
              <div>
                <label className="text-xs text-slate-300 block mb-1">Find string in payload:</label>
                <input
                  type="text"
                  value={findText}
                  onChange={(e) => setFindText(e.target.value)}
                  placeholder="e.g. http://old-api.internal"
                  className="w-full font-mono text-xs px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-200 focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div>
                <label className="text-xs text-slate-300 block mb-1">Replace with:</label>
                <input
                  type="text"
                  value={replaceText}
                  onChange={(e) => setReplaceText(e.target.value)}
                  placeholder="e.g. https://api.production.internal"
                  className="w-full font-mono text-xs px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-200 focus:outline-none focus:border-indigo-500"
                />
              </div>
              <div className="flex items-center justify-end gap-2 pt-2">
                <button
                  type="button"
                  onClick={() => setTransformCluster(null)}
                  className="px-3 py-1.5 text-xs text-slate-400 hover:text-slate-200"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  disabled={transforming || !findText.trim()}
                  className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-xs font-semibold bg-indigo-500 hover:bg-indigo-400 text-white transition-all disabled:opacity-50"
                >
                  <Check className="h-3.5 w-3.5" />
                  {transforming ? 'Transforming...' : 'Transform & Replay'}
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
