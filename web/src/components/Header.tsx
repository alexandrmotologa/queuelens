import React from 'react';
import { Layers, Radio, AlertOctagon, ShieldAlert, RefreshCw, Database, Bell } from 'lucide-react';
import { ServerStatus } from '../types';

interface HeaderProps {
  status: ServerStatus | null;
  activeTab: 'overview' | 'stream' | 'dlq' | 'stalls';
  setActiveTab: (tab: 'overview' | 'stream' | 'dlq' | 'stalls') => void;
  wsConnected: boolean;
  onRefresh: () => void;
  onOpenConnect: () => void;
  onOpenAlerts: () => void;
  stallCount: number;
  dlqCount: number;
}

export const Header: React.FC<HeaderProps> = ({
  status,
  activeTab,
  setActiveTab,
  wsConnected,
  onRefresh,
  onOpenConnect,
  onOpenAlerts,
  stallCount,
  dlqCount,
}) => {
  return (
    <header className="border-b border-slate-800 bg-[#0c1222]/95 backdrop-blur sticky top-0 z-40 px-4 lg:px-8 py-3">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row items-center justify-between gap-3 md:gap-4">
        {/* Brand */}
        <div className="flex items-center justify-between w-full md:w-auto shrink-0">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 rounded-xl bg-gradient-to-tr from-sky-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-sky-500/20 shrink-0">
              <Layers className="h-5 w-5 text-white" />
            </div>
            <div className="flex flex-col">
              <div className="flex items-center gap-2">
                <span className="font-bold text-base tracking-tight text-white whitespace-nowrap">QueueLens</span>
                <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-sky-500/10 text-sky-400 border border-sky-500/20 whitespace-nowrap">
                  v1.1.0
                </span>
                {status?.isDemo && (
                  <span className="text-[10px] font-mono uppercase font-bold tracking-wider px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/30 whitespace-nowrap">
                    DEMO MODE
                  </span>
                )}
              </div>
              <span className="text-xs text-slate-400 whitespace-nowrap hidden sm:block">
                Universal Task Queue Observability & DLQ Studio
              </span>
            </div>
          </div>

          {/* Quick status for narrow mobile viewports */}
          <div className="flex md:hidden items-center gap-2 shrink-0">
            <div className="h-8 inline-flex items-center gap-1.5 px-2.5 rounded-lg bg-slate-900 border border-slate-800 text-[10px] font-mono text-slate-300">
              <span
                className={`h-2 w-2 rounded-full ${
                  wsConnected ? 'bg-emerald-400 shadow-sm shadow-emerald-500/50' : 'bg-red-400 animate-pulse'
                }`}
              />
              <span>{wsConnected ? 'LIVE' : 'DISC'}</span>
            </div>
            <button
              onClick={onRefresh}
              title="Refresh Data"
              className="h-8 w-8 inline-flex items-center justify-center rounded-lg bg-slate-900 border border-slate-800 text-slate-400 hover:text-white"
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>

        {/* Navigation Tabs - Strict Single Line with Identical Button Heights */}
        <nav className="flex items-center gap-1 p-1 rounded-xl bg-slate-900/90 border border-slate-800 shrink-0 max-w-full overflow-x-auto no-scrollbar">
          <button
            onClick={() => setActiveTab('overview')}
            className={`h-9 inline-flex items-center gap-2 px-3.5 rounded-lg text-xs whitespace-nowrap select-none transition-all shrink-0 ${
              activeTab === 'overview'
                ? 'bg-sky-500 text-white font-semibold shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 font-medium'
            }`}
          >
            <Layers className="h-3.5 w-3.5 shrink-0" />
            <span>Overview</span>
          </button>

          <button
            onClick={() => setActiveTab('stream')}
            className={`h-9 inline-flex items-center gap-2 px-3.5 rounded-lg text-xs whitespace-nowrap select-none transition-all shrink-0 ${
              activeTab === 'stream'
                ? 'bg-sky-500 text-white font-semibold shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 font-medium'
            }`}
          >
            <Radio className="h-3.5 w-3.5 shrink-0" />
            <span>Live Stream</span>
          </button>

          <button
            onClick={() => setActiveTab('dlq')}
            className={`h-9 inline-flex items-center gap-2 px-3.5 rounded-lg text-xs whitespace-nowrap select-none transition-all shrink-0 ${
              activeTab === 'dlq'
                ? 'bg-sky-500 text-white font-semibold shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 font-medium'
            }`}
          >
            <AlertOctagon className="h-3.5 w-3.5 shrink-0" />
            <span>DLQ Studio</span>
            {dlqCount > 0 && (
              <span className="px-1.5 py-0.5 rounded-full text-[10px] font-mono font-bold bg-red-500/20 text-red-400 border border-red-500/30">
                {dlqCount}
              </span>
            )}
          </button>

          <button
            onClick={() => setActiveTab('stalls')}
            className={`h-9 inline-flex items-center gap-2 px-3.5 rounded-lg text-xs whitespace-nowrap select-none transition-all shrink-0 ${
              activeTab === 'stalls'
                ? 'bg-sky-500 text-white font-semibold shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60 font-medium'
            }`}
          >
            <ShieldAlert className="h-3.5 w-3.5 shrink-0" />
            <span>Stalls</span>
            {stallCount > 0 && (
              <span className="px-1.5 py-0.5 rounded-full text-[10px] font-mono font-bold bg-amber-500/20 text-amber-400 border border-amber-500/30">
                {stallCount}
              </span>
            )}
          </button>
        </nav>

        {/* Controls - Uniform Height & Spacing */}
        <div className="hidden md:flex items-center gap-2 shrink-0">
          {/* Active Connection Switcher */}
          <button
            onClick={onOpenConnect}
            title="Click to Switch Redis Host"
            className="h-9 inline-flex items-center gap-2 px-3 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 hover:border-slate-700 text-xs text-slate-300 transition-all whitespace-nowrap shrink-0 cursor-pointer shadow-sm"
          >
            <Database className="h-3.5 w-3.5 text-sky-400 shrink-0" />
            <span className="font-mono text-slate-300 max-w-[150px] truncate">
              {status?.redisAddr || '127.0.0.1:6379'}
            </span>
          </button>

          {/* Webhook Alerts Button */}
          <button
            onClick={onOpenAlerts}
            title="Configure Alert Webhooks"
            className="h-9 w-9 inline-flex items-center justify-center rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-400 hover:text-amber-400 transition-colors shrink-0"
          >
            <Bell className="h-4 w-4" />
          </button>

          {/* Live indicator */}
          <div className="h-9 inline-flex items-center gap-2 px-3 rounded-lg bg-slate-900 border border-slate-800 text-xs font-mono text-slate-400 shrink-0 whitespace-nowrap">
            <span
              className={`h-2 w-2 rounded-full ${
                wsConnected ? 'bg-emerald-400 shadow-sm shadow-emerald-500/50' : 'bg-red-400 animate-pulse'
              }`}
            />
            <span className="text-[11px] font-semibold text-slate-300">{wsConnected ? 'LIVE' : 'DISC'}</span>
          </div>

          {/* Refresh Button */}
          <button
            onClick={onRefresh}
            title="Refresh Data (HotKey: R)"
            className="h-9 w-9 inline-flex items-center justify-center rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-400 hover:text-white transition-colors shrink-0"
          >
            <RefreshCw className="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>
  );
};
