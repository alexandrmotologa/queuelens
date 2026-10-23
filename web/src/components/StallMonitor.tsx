import React, { useState } from 'react';
import { StallWarning, QueueType } from '../types';
import { ShieldAlert, Unlock, CheckCircle, Clock, Key, Cpu } from 'lucide-react';

interface StallMonitorProps {
  stalls: StallWarning[];
  onForceUnlock: (queueType: QueueType, queue: string, lockKey: string) => Promise<void>;
  onInspectJob: (queueType: string, queue: string, jobId: string) => void;
}

export const StallMonitor: React.FC<StallMonitorProps> = ({
  stalls,
  onForceUnlock,
  onInspectJob,
}) => {
  const [unlockingKey, setUnlockingKey] = useState<string | null>(null);

  const handleUnlock = async (stall: StallWarning) => {
    setUnlockingKey(stall.lockKey);
    try {
      await onForceUnlock(stall.queueType, stall.queue, stall.lockKey);
    } finally {
      setUnlockingKey(null);
    }
  };

  return (
    <div className="space-y-4">
      {/* Overview Banner */}
      <div className="p-4 rounded-xl bg-slate-900/80 border border-slate-800 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <div className="p-2.5 rounded-lg bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <ShieldAlert className="h-5 w-5" />
          </div>
          <div>
            <h2 className="text-base font-semibold text-white">Worker Lock & Stall Auditor</h2>
            <p className="text-xs text-slate-400 mt-0.5">
              Identifies active jobs holding locks where the worker crashed or lock lease expired
            </p>
          </div>
        </div>

        <div className="text-xs font-mono px-3 py-1.5 rounded-lg bg-slate-800 border border-slate-700 text-slate-300">
          Detected Stalls: <span className="text-amber-400 font-bold">{stalls.length}</span>
        </div>
      </div>

      {stalls.length === 0 ? (
        <div className="p-16 text-center rounded-xl bg-slate-900/40 border border-slate-800">
          <CheckCircle className="h-10 w-10 text-emerald-500 mx-auto mb-3" />
          <h3 className="text-sm font-medium text-slate-200">All Worker Locks Healthy</h3>
          <p className="text-xs text-slate-500 mt-1 max-w-sm mx-auto">
            No orphaned locks or stalled background workers detected across your queues.
          </p>
        </div>
      ) : (
        <div className="rounded-xl border border-slate-800 bg-slate-900/60 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-left text-xs">
              <thead className="bg-slate-900/90 text-slate-400 border-b border-slate-800 text-[11px] font-mono uppercase tracking-wider">
                <tr>
                  <th className="py-3 px-4">Job ID</th>
                  <th className="py-3 px-4">Queue</th>
                  <th className="py-3 px-4">Lock Key</th>
                  <th className="py-3 px-4">Status & Reason</th>
                  <th className="py-3 px-4">Active Duration</th>
                  <th className="py-3 px-4 text-right">Remediation</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-800/60">
                {stalls.map((stall) => (
                  <tr key={`${stall.queue}-${stall.jobId}`} className="hover:bg-slate-800/40 transition-colors">
                    <td className="py-3 px-4 font-mono text-slate-200">
                      <span className="bg-slate-800 px-2 py-0.5 rounded border border-slate-700 text-sky-400">
                        {stall.jobId}
                      </span>
                    </td>
                    <td className="py-3 px-4">
                      <span className="font-medium text-slate-300">{stall.queue}</span>
                      <span className="ml-1 text-[10px] text-slate-500 font-mono">({stall.queueType})</span>
                    </td>
                    <td className="py-3 px-4 font-mono text-slate-400 max-w-[220px] truncate" title={stall.lockKey}>
                      <span className="flex items-center gap-1.5">
                        <Key className="h-3 w-3 text-amber-500" />
                        {stall.lockKey}
                      </span>
                    </td>
                    <td className="py-3 px-4">
                      <span className="text-amber-400 bg-amber-500/10 border border-amber-500/20 px-2 py-0.5 rounded text-[11px] font-mono">
                        {stall.reason}
                      </span>
                    </td>
                    <td className="py-3 px-4 font-mono text-slate-400 whitespace-nowrap">
                      <span className="flex items-center gap-1">
                        <Clock className="h-3 w-3 text-slate-500" />
                        {Math.floor(stall.elapsedMs / 1000)}s elapsed
                      </span>
                    </td>
                    <td className="py-3 px-4 text-right whitespace-nowrap">
                      <div className="flex items-center justify-end gap-2">
                        <button
                          onClick={() => onInspectJob(stall.queueType, stall.queue, stall.jobId)}
                          className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-slate-300 transition-colors"
                        >
                          View
                        </button>
                        <button
                          disabled={unlockingKey === stall.lockKey}
                          onClick={() => handleUnlock(stall)}
                          className="flex items-center gap-1 px-3 py-1 rounded bg-amber-500/20 hover:bg-amber-500/30 text-amber-400 border border-amber-500/30 transition-colors font-medium text-xs disabled:opacity-50"
                        >
                          <Unlock className="h-3 w-3" />
                          {unlockingKey === stall.lockKey ? 'Releasing...' : 'Force Unlock'}
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
};
