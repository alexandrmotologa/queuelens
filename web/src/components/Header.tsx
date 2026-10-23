import React from 'react';
import { Layers, Activity, AlertOctagon, ShieldAlert, RefreshCw, Radio, Database } from 'lucide-react';
import { ServerStatus } from '../types';

interface HeaderProps {
  status: ServerStatus | null;
  activeTab: 'overview' | 'stream' | 'dlq' | 'stalls';
  setActiveTab: (tab: 'overview' | 'stream' | 'dlq' | 'stalls') => void;
  wsConnected: boolean;
  onRefresh: () => void;
  stallCount: number;
  dlqCount: number;
}

export const Header: React.FC<HeaderProps> = ({
  status,
  activeTab,
  setActiveTab,
  wsConnected,
  onRefresh,
  stallCount,
  dlqCount,
}) => {
  return (
    <header className="border-b border-slate-800 bg-[#0c1222]/90 backdrop-blur sticky top-0 z-40 px-6 py-3.5">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row items-center justify-between gap-4">
        {/* Brand */}
        <div className="flex items-center gap-3">
          <div className="h-10 w-10 rounded-xl bg-gradient-to-tr from-sky-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-sky-500/20">
            <Layers className="h-5 w-5 text-white" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <h1 className="font-bold text-lg tracking-tight text-white">QueueLens</h1>
              <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-sky-500/10 text-sky-400 border border-sky-500/20">
                v1.0.0
              </span>
              {status?.isDemo && (
                <span className="text-[10px] font-mono uppercase font-bold tracking-wider px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/30 animate-pulse">
                  Demo Simulation
                </span>
              )}
            </div>
            <p className="text-xs text-slate-400">
              Multi-Framework Background Queue Observability & DLQ Cockpit
            </p>
          </div>
        </div>

        {/* Navigation Tabs */}
        <nav className="flex items-center gap-1.5 p-1 rounded-xl bg-slate-900/80 border border-slate-800">
          <button
            onClick={() => setActiveTab('overview')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all ${
              activeTab === 'overview'
                ? 'bg-sky-500 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Layers className="h-3.5 w-3.5" />
            Overview
          </button>
          <button
            onClick={() => setActiveTab('stream')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all ${
              activeTab === 'stream'
                ? 'bg-sky-500 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Radio className="h-3.5 w-3.5" />
            Live Stream
          </button>
          <button
            onClick={() => setActiveTab('dlq')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all relative ${
              activeTab === 'dlq'
                ? 'bg-sky-500 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <AlertOctagon className="h-3.5 w-3.5" />
            DLQ Studio
            {dlqCount > 0 && (
              <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] bg-red-500/20 text-red-400 border border-red-500/30">
                {dlqCount}
              </span>
            )}
          </button>
          <button
            onClick={() => setActiveTab('stalls')}
            className={`flex items-center gap-2 px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all relative ${
              activeTab === 'stalls'
                ? 'bg-sky-500 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <ShieldAlert className="h-3.5 w-3.5" />
            Stall Monitor
            {stallCount > 0 && (
              <span className="ml-1 px-1.5 py-0.2 rounded-full text-[10px] bg-amber-500/20 text-amber-400 border border-amber-500/30">
                {stallCount}
              </span>
            )}
          </button>
        </nav>

        {/* Status Indicators */}
        <div className="flex items-center gap-3">
          <div className="hidden lg:flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800 text-xs text-slate-300">
            <Database className="h-3.5 w-3.5 text-sky-400" />
            <span className="font-mono text-slate-400">{status?.redisAddr || 'redis://localhost:6379'}</span>
            {status?.server?.version && (
              <span className="text-[10px] text-slate-500">v{status.server.version}</span>
            )}
          </div>

          <div className="flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] bg-slate-900 border border-slate-800">
            <span
              className={`h-2 w-2 rounded-full ${
                wsConnected ? 'bg-emerald-400 shadow-sm shadow-emerald-500/50' : 'bg-red-400 animate-pulse'
              }`}
            />
            <span className="text-slate-400">{wsConnected ? 'Live' : 'Disconnected'}</span>
          </div>

          <button
            onClick={onRefresh}
            title="Refresh All Data"
            className="p-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
          >
            <RefreshCw className="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>
  );
};
