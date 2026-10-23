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
    <header className="border-b border-slate-800 bg-[#0c1222]/95 backdrop-blur sticky top-0 z-40 px-4 sm:px-6 py-3">
      <div className="max-w-7xl mx-auto flex flex-col md:flex-row items-center justify-between gap-3.5">
        {/* Brand */}
        <div className="flex items-center justify-between w-full md:w-auto gap-3">
          <div className="flex items-center gap-3">
            <div className="h-9 w-9 sm:h-10 sm:w-10 rounded-xl bg-gradient-to-tr from-sky-500 to-indigo-600 flex items-center justify-center shadow-lg shadow-sky-500/20 shrink-0">
              <Layers className="h-5 w-5 text-white" />
            </div>
            <div>
              <div className="flex items-center gap-2">
                <h1 className="font-bold text-base sm:text-lg tracking-tight text-white">QueueLens</h1>
                <span className="text-[10px] font-mono px-2 py-0.5 rounded-full bg-sky-500/10 text-sky-400 border border-sky-500/20">
                  v1.1.0
                </span>
                {status?.isDemo && (
                  <span className="text-[10px] font-mono uppercase font-bold tracking-wider px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/30 animate-pulse">
                    Demo
                  </span>
                )}
              </div>
              <p className="text-[11px] sm:text-xs text-slate-400">
                Universal Task Queue Observability & DLQ Studio
              </p>
            </div>
          </div>

          {/* Mobile Right Quick Status */}
          <div className="flex md:hidden items-center gap-2">
            <div className="flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] bg-slate-900 border border-slate-800">
              <span
                className={`h-2 w-2 rounded-full ${
                  wsConnected ? 'bg-emerald-400 shadow-sm shadow-emerald-500/50' : 'bg-red-400 animate-pulse'
                }`}
              />
              <span className="text-slate-400 font-mono">{wsConnected ? 'LIVE' : 'DISC'}</span>
            </div>
            <button
              onClick={onRefresh}
              title="Refresh"
              className="p-1.5 rounded-lg bg-slate-900 border border-slate-800 text-slate-400 hover:text-white"
            >
              <RefreshCw className="h-3.5 w-3.5" />
            </button>
          </div>
        </div>

        {/* Navigation Tabs (Smooth Horizontal Scroll on Mobile) */}
        <nav className="flex items-center gap-1 sm:gap-1.5 p-1 rounded-xl bg-slate-900/90 border border-slate-800 max-w-full overflow-x-auto no-scrollbar scroll-smooth">
          <button
            onClick={() => setActiveTab('overview')}
            className={`flex items-center gap-1.5 sm:gap-2 px-3 sm:px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all shrink-0 ${
              activeTab === 'overview'
                ? 'bg-sky-500 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Layers className="h-3.5 w-3.5" />
            <span>Overview</span>
            <kbd className="hidden lg:inline-block text-[10px] opacity-60 bg-black/20 px-1 rounded">1</kbd>
          </button>
          <button
            onClick={() => setActiveTab('stream')}
            className={`flex items-center gap-1.5 sm:gap-2 px-3 sm:px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all shrink-0 ${
              activeTab === 'stream'
                ? 'bg-sky-500 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <Radio className="h-3.5 w-3.5" />
            <span>Live Stream</span>
            <kbd className="hidden lg:inline-block text-[10px] opacity-60 bg-black/20 px-1 rounded">2</kbd>
          </button>
          <button
            onClick={() => setActiveTab('dlq')}
            className={`flex items-center gap-1.5 sm:gap-2 px-3 sm:px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all relative shrink-0 ${
              activeTab === 'dlq'
                ? 'bg-sky-500 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <AlertOctagon className="h-3.5 w-3.5" />
            <span>DLQ Studio</span>
            {dlqCount > 0 && (
              <span className="ml-0.5 px-1.5 py-0.2 rounded-full text-[10px] bg-red-500/20 text-red-400 border border-red-500/30">
                {dlqCount}
              </span>
            )}
            <kbd className="hidden lg:inline-block text-[10px] opacity-60 bg-black/20 px-1 rounded">3</kbd>
          </button>
          <button
            onClick={() => setActiveTab('stalls')}
            className={`flex items-center gap-1.5 sm:gap-2 px-3 sm:px-3.5 py-1.5 rounded-lg text-xs font-medium transition-all relative shrink-0 ${
              activeTab === 'stalls'
                ? 'bg-sky-500 text-white shadow-sm'
                : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/60'
            }`}
          >
            <ShieldAlert className="h-3.5 w-3.5" />
            <span>Stalls</span>
            {stallCount > 0 && (
              <span className="ml-0.5 px-1.5 py-0.2 rounded-full text-[10px] bg-amber-500/20 text-amber-400 border border-amber-500/30">
                {stallCount}
              </span>
            )}
            <kbd className="hidden lg:inline-block text-[10px] opacity-60 bg-black/20 px-1 rounded">4</kbd>
          </button>
        </nav>

        {/* Desktop / Tablet Controls */}
        <div className="flex flex-wrap items-center justify-center gap-2 sm:gap-2.5">
          {/* Active Connection Switcher */}
          <button
            onClick={onOpenConnect}
            title="Click to Switch Redis Host"
            className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-900 hover:bg-slate-850 border border-slate-800 hover:border-slate-700 text-xs text-slate-300 transition-all cursor-pointer shadow-sm"
          >
            <Database className="h-3.5 w-3.5 text-sky-400 shrink-0" />
            <span className="font-mono text-slate-300 max-w-[130px] sm:max-w-[160px] truncate">
              {status?.redisAddr || '127.0.0.1:6379'}
            </span>
          </button>

          {/* Webhook Alerts Button */}
          <button
            onClick={onOpenAlerts}
            title="Configure Alert Webhooks"
            className="p-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-400 hover:text-amber-400 transition-colors"
          >
            <Bell className="h-4 w-4" />
          </button>

          {/* Live indicator (Hidden on tiny mobile since rendered above) */}
          <div className="hidden md:flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[11px] bg-slate-900 border border-slate-800">
            <span
              className={`h-2 w-2 rounded-full ${
                wsConnected ? 'bg-emerald-400 shadow-sm shadow-emerald-500/50' : 'bg-red-400 animate-pulse'
              }`}
            />
            <span className="text-slate-400 font-mono text-[10px]">{wsConnected ? 'LIVE' : 'DISC'}</span>
          </div>

          {/* Refresh Button */}
          <button
            onClick={onRefresh}
            title="Refresh (HotKey: R)"
            className="hidden md:flex p-1.5 rounded-lg bg-slate-900 hover:bg-slate-800 border border-slate-800 text-slate-400 hover:text-slate-200 transition-colors"
          >
            <RefreshCw className="h-4 w-4" />
          </button>
        </div>
      </div>
    </header>
  );
};
