import React, { useState } from 'react';
import { X, Database, Check, Server, Shield, ArrowRight } from 'lucide-react';

interface ConnectModalProps {
  isOpen: boolean;
  onClose: () => void;
  currentAddr: string;
  onConnect: (url: string) => Promise<void>;
}

export const ConnectModal: React.FC<ConnectModalProps> = ({
  isOpen,
  onClose,
  currentAddr,
  onConnect,
}) => {
  if (!isOpen) return null;

  const [url, setUrl] = useState(currentAddr || 'redis://127.0.0.1:6379/0');
  const [connecting, setConnecting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const presets = [
    { label: 'Local Redis (Standalone)', url: 'redis://127.0.0.1:6379/0' },
    { label: 'Local Redis with Password', url: 'redis://:secret@127.0.0.1:6379/0' },
    { label: 'Valkey Instance', url: 'redis://127.0.0.1:6380/0' },
    { label: 'Staging / Remote Cluster', url: 'rediss://staging-redis.internal:6379' },
  ];

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setConnecting(true);
    setError(null);
    try {
      await onConnect(url);
      onClose();
    } catch (err: any) {
      setError(err.message || 'Failed to connect to Redis instance');
    } finally {
      setConnecting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div className="w-full max-w-lg rounded-2xl bg-slate-900 border border-slate-800 shadow-2xl overflow-hidden flex flex-col">
        {/* Header */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/60">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-sky-500/10 text-sky-400 border border-sky-500/20">
              <Database className="h-5 w-5" />
            </div>
            <div>
              <h3 className="font-semibold text-slate-100 text-sm">Switch Redis / Valkey Connection</h3>
              <p className="text-xs text-slate-400">Hot-swap active message broker without restarting QueueLens</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-1.5 rounded-lg text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSubmit} className="p-5 space-y-4">
          <div>
            <label className="text-xs font-semibold text-slate-300 block mb-1.5">
              Connection URL (TCP, TLS or Auth)
            </label>
            <input
              type="text"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="redis://:password@host:6379/0"
              className="w-full font-mono text-xs px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-sky-500 focus:ring-1 focus:ring-sky-500 transition-all"
            />
          </div>

          {/* Presets */}
          <div>
            <span className="text-[11px] font-mono text-slate-400 uppercase tracking-wider block mb-2">
              Quick Connect Presets:
            </span>
            <div className="space-y-1.5">
              {presets.map((preset) => (
                <button
                  type="button"
                  key={preset.url}
                  onClick={() => setUrl(preset.url)}
                  className="w-full text-left p-2.5 rounded-lg bg-slate-950/60 hover:bg-slate-800 border border-slate-800/80 hover:border-slate-700 transition-all flex items-center justify-between text-xs"
                >
                  <span className="font-medium text-slate-300">{preset.label}</span>
                  <span className="font-mono text-slate-500 text-[11px] truncate max-w-[200px]">{preset.url}</span>
                </button>
              ))}
            </div>
          </div>

          {error && (
            <div className="p-3 rounded-lg bg-red-950/40 border border-red-900/50 text-red-400 text-xs font-mono">
              {error}
            </div>
          )}

          {/* Footer */}
          <div className="pt-2 flex items-center justify-end gap-2 border-t border-slate-800">
            <button
              type="button"
              onClick={onClose}
              className="px-3.5 py-1.5 rounded-lg text-xs text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={connecting || !url.trim()}
              className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-xs font-semibold bg-sky-500 hover:bg-sky-400 text-white transition-all disabled:opacity-50 shadow-sm shadow-sky-500/20"
            >
              <Check className="h-3.5 w-3.5" />
              {connecting ? 'Connecting...' : 'Connect Now'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
};
