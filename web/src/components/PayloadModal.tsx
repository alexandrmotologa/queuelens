import React, { useState, useEffect } from 'react';
import { Job } from '../types';
import { X, Copy, Check, RotateCw, Trash2, Code, Clock, Layers, ShieldAlert } from 'lucide-react';

interface PayloadModalProps {
  job: Job | null;
  onClose: () => void;
  onRetry: (job: Job, mutatedPayload: string) => Promise<void>;
  onDelete: (job: Job) => Promise<void>;
}

export const PayloadModal: React.FC<PayloadModalProps> = ({
  job,
  onClose,
  onRetry,
  onDelete,
}) => {
  if (!job) return null;

  const [payloadStr, setPayloadStr] = useState<string>('');
  const [copied, setCopied] = useState<boolean>(false);
  const [jsonError, setJsonError] = useState<string | null>(null);
  const [isRetrying, setIsRetrying] = useState<boolean>(false);
  const [isDeleting, setIsDeleting] = useState<boolean>(false);

  useEffect(() => {
    try {
      const formatted = JSON.stringify(job.payload, null, 2);
      setPayloadStr(formatted || '{}');
      setJsonError(null);
    } catch {
      setPayloadStr(job.rawPayload || '{}');
    }
  }, [job]);

  const handleCopy = () => {
    navigator.clipboard.writeText(payloadStr);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const handlePayloadChange = (e: React.ChangeEvent<HTMLTextAreaElement>) => {
    const val = e.target.value;
    setPayloadStr(val);
    try {
      JSON.parse(val);
      setJsonError(null);
    } catch (err: any) {
      setJsonError(err.message);
    }
  };

  const handleRetry = async () => {
    if (jsonError) return;
    setIsRetrying(true);
    try {
      await onRetry(job, payloadStr);
      onClose();
    } finally {
      setIsRetrying(false);
    }
  };

  const handleDelete = async () => {
    if (!window.confirm(`Are you sure you want to delete job ${job.id}?`)) return;
    setIsDeleting(true);
    try {
      await onDelete(job);
      onClose();
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div className="w-full max-w-3xl rounded-2xl bg-slate-900 border border-slate-800 shadow-2xl overflow-hidden flex flex-col max-h-[90vh]">
        {/* Header */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/60">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-sky-500/10 text-sky-400 border border-sky-500/20">
              <Code className="h-5 w-5" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h3 className="font-semibold text-slate-100 text-sm">{job.name}</h3>
                <span className="font-mono text-xs text-sky-400 bg-slate-800 px-2 py-0.5 rounded border border-slate-700">
                  {job.id}
                </span>
                <span className="text-[10px] uppercase font-mono px-2 py-0.5 rounded bg-slate-800 text-slate-400">
                  {job.state}
                </span>
              </div>
              <p className="text-xs text-slate-400 font-mono mt-0.5">
                Queue: {job.queue} ({job.queueType})
              </p>
            </div>
          </div>

          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Content body */}
        <div className="p-5 overflow-y-auto space-y-4 flex-1">
          {/* Metadata badges */}
          <div className="grid grid-cols-2 sm:grid-cols-4 gap-2 text-xs font-mono">
            <div className="p-2.5 rounded-lg bg-slate-950/60 border border-slate-800">
              <span className="text-slate-500 block text-[10px]">Attempts:</span>
              <span className="text-slate-200 font-semibold">{job.attempts} / {job.maxAttempts}</span>
            </div>
            <div className="p-2.5 rounded-lg bg-slate-950/60 border border-slate-800">
              <span className="text-slate-500 block text-[10px]">Enqueued:</span>
              <span className="text-slate-200">{job.enqueuedAt ? new Date(job.enqueuedAt).toLocaleTimeString() : '-'}</span>
            </div>
            <div className="p-2.5 rounded-lg bg-slate-950/60 border border-slate-800">
              <span className="text-slate-500 block text-[10px]">Wait Latency:</span>
              <span className="text-slate-200">{job.waitTimeMs ? `${job.waitTimeMs}ms` : '-'}</span>
            </div>
            <div className="p-2.5 rounded-lg bg-slate-950/60 border border-slate-800">
              <span className="text-slate-500 block text-[10px]">Execution Time:</span>
              <span className="text-slate-200">{job.durationMs ? `${job.durationMs}ms` : '-'}</span>
            </div>
          </div>

          {/* Failure reason if any */}
          {job.errorReason && (
            <div className="p-3.5 rounded-xl bg-red-950/30 border border-red-900/50">
              <div className="text-xs font-semibold text-red-400 mb-1 flex items-center gap-1.5">
                <ShieldAlert className="h-4 w-4" />
                Failure Reason
              </div>
              <div className="font-mono text-xs text-red-300 whitespace-pre-wrap">{job.errorReason}</div>
              {job.stackTrace && (
                <div className="mt-2 text-[11px] font-mono text-slate-400 max-h-36 overflow-y-auto bg-slate-950 p-2.5 rounded border border-slate-800">
                  {job.stackTrace}
                </div>
              )}
            </div>
          )}

          {/* JSON Payload Editor */}
          <div>
            <div className="flex items-center justify-between mb-2">
              <label className="text-xs font-semibold text-slate-300">
                Payload JSON (In-Place Mutation Supported)
              </label>
              <button
                onClick={handleCopy}
                className="flex items-center gap-1 text-xs text-slate-400 hover:text-slate-200 transition-colors"
              >
                {copied ? <Check className="h-3.5 w-3.5 text-emerald-400" /> : <Copy className="h-3.5 w-3.5" />}
                {copied ? 'Copied' : 'Copy JSON'}
              </button>
            </div>

            <textarea
              rows={10}
              value={payloadStr}
              onChange={handlePayloadChange}
              className="w-full font-mono text-xs p-3 rounded-xl bg-slate-950 border border-slate-800 text-slate-200 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500 transition-all"
            />

            {jsonError && (
              <div className="text-xs text-red-400 mt-1 font-mono">
                Syntax Error: {jsonError}
              </div>
            )}
          </div>
        </div>

        {/* Footer Actions */}
        <div className="p-4 border-t border-slate-800 bg-slate-950/60 flex items-center justify-between">
          <button
            disabled={isDeleting}
            onClick={handleDelete}
            className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs text-red-400 hover:bg-red-500/10 border border-red-500/20 transition-colors disabled:opacity-50"
          >
            <Trash2 className="h-3.5 w-3.5" />
            {isDeleting ? 'Deleting...' : 'Delete Job'}
          </button>

          <div className="flex items-center gap-2">
            <button
              onClick={onClose}
              className="px-3.5 py-1.5 rounded-lg text-xs text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
            >
              Cancel
            </button>
            <button
              disabled={isRetrying || !!jsonError}
              onClick={handleRetry}
              className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-xs font-semibold bg-sky-500 hover:bg-sky-400 text-white transition-all disabled:opacity-50 shadow-sm shadow-sky-500/20"
            >
              <RotateCw className={`h-3.5 w-3.5 ${isRetrying ? 'animate-spin' : ''}`} />
              {isRetrying ? 'Retrying...' : 'Save & Retry Job'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
