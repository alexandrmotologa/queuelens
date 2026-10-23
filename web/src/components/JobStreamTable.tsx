import React, { useState } from 'react';
import { JobEvent, JobState, QueueInfo } from '../types';
import { Filter, Search, Eye, ArrowRight, CheckCircle2, XCircle, Clock, Zap, Calendar } from 'lucide-react';

interface JobStreamTableProps {
  events: JobEvent[];
  queues: QueueInfo[];
  onInspectJob: (queueType: string, queue: string, id: string) => void;
}

export const JobStreamTable: React.FC<JobStreamTableProps> = ({
  events,
  queues,
  onInspectJob,
}) => {
  const [selectedQueue, setSelectedQueue] = useState<string>('all');
  const [selectedState, setSelectedState] = useState<string>('all');
  const [timeWindow, setTimeWindow] = useState<string>('all'); // all, 15m, 1h, 24h
  const [searchQuery, setSearchQuery] = useState<string>('');

  const now = Date.now();
  const filteredEvents = events.filter((ev) => {
    if (selectedQueue !== 'all' && ev.queue !== selectedQueue) return false;
    if (selectedState !== 'all' && ev.toState !== selectedState) return false;

    // Time window filter
    if (timeWindow !== 'all') {
      const eventTime = new Date(ev.timestamp).getTime();
      const diffMins = (now - eventTime) / (1000 * 60);
      if (timeWindow === '15m' && diffMins > 15) return false;
      if (timeWindow === '1h' && diffMins > 60) return false;
      if (timeWindow === '24h' && diffMins > 1440) return false;
    }

    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase();
      const matchID = ev.id.toLowerCase().includes(q);
      const matchName = ev.name.toLowerCase().includes(q);
      const matchError = ev.errorReason?.toLowerCase().includes(q);
      return matchID || matchName || matchError;
    }
    return true;
  });

  const getStateBadge = (state: JobState) => {
    switch (state) {
      case 'waiting':
        return (
          <span className="flex items-center gap-1 text-[11px] font-mono px-2 py-0.5 rounded-full bg-indigo-500/10 text-indigo-400 border border-indigo-500/20">
            <Clock className="h-3 w-3" />
            WAITING
          </span>
        );
      case 'active':
        return (
          <span className="flex items-center gap-1 text-[11px] font-mono px-2 py-0.5 rounded-full bg-sky-500/10 text-sky-400 border border-sky-500/20 animate-pulse">
            <Zap className="h-3 w-3" />
            ACTIVE
          </span>
        );
      case 'completed':
        return (
          <span className="flex items-center gap-1 text-[11px] font-mono px-2 py-0.5 rounded-full bg-emerald-500/10 text-emerald-400 border border-emerald-500/20">
            <CheckCircle2 className="h-3 w-3" />
            COMPLETED
          </span>
        );
      case 'failed':
        return (
          <span className="flex items-center gap-1 text-[11px] font-mono px-2 py-0.5 rounded-full bg-red-500/10 text-red-400 border border-red-500/20">
            <XCircle className="h-3 w-3" />
            FAILED
          </span>
        );
      case 'delayed':
        return (
          <span className="flex items-center gap-1 text-[11px] font-mono px-2 py-0.5 rounded-full bg-amber-500/10 text-amber-400 border border-amber-500/20">
            <Clock className="h-3 w-3" />
            DELAYED
          </span>
        );
      default:
        return null;
    }
  };

  return (
    <div className="space-y-4">
      {/* Controls / Filter Bar */}
      <div className="flex flex-col md:flex-row items-center justify-between gap-3 p-3.5 rounded-xl bg-slate-900/70 border border-slate-800">
        <div className="flex flex-wrap items-center gap-2 w-full md:w-auto flex-1">
          <div className="relative flex-1 min-w-[200px] md:max-w-xs">
            <Search className="absolute left-3 top-2.5 h-4 w-4 text-slate-500" />
            <input
              id="stream-search"
              type="text"
              placeholder="Search ID, name, or error (Press '/' to focus)..."
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              className="w-full pl-9 pr-3 py-1.5 text-xs rounded-lg bg-slate-800/80 border border-slate-700 text-slate-100 placeholder-slate-500 focus:outline-none focus:border-sky-500 transition-colors"
            />
          </div>

          <div className="flex items-center gap-1.5">
            <Filter className="h-3.5 w-3.5 text-slate-500" />
            <select
              value={selectedQueue}
              onChange={(e) => setSelectedQueue(e.target.value)}
              className="px-2.5 py-1.5 text-xs rounded-lg bg-slate-800/80 border border-slate-700 text-slate-200 focus:outline-none focus:border-sky-500"
            >
              <option value="all">All Queues</option>
              {queues.map((q) => (
                <option key={q.name} value={q.name}>
                  {q.name} ({q.type})
                </option>
              ))}
            </select>

            <select
              value={selectedState}
              onChange={(e) => setSelectedState(e.target.value)}
              className="px-2.5 py-1.5 text-xs rounded-lg bg-slate-800/80 border border-slate-700 text-slate-200 focus:outline-none focus:border-sky-500"
            >
              <option value="all">All States</option>
              <option value="waiting">Waiting</option>
              <option value="active">Active</option>
              <option value="completed">Completed</option>
              <option value="failed">Failed</option>
            </select>

            <div className="flex items-center gap-1 pl-1">
              <Calendar className="h-3.5 w-3.5 text-slate-500" />
              <select
                value={timeWindow}
                onChange={(e) => setTimeWindow(e.target.value)}
                className="px-2.5 py-1.5 text-xs rounded-lg bg-slate-800/80 border border-slate-700 text-slate-200 focus:outline-none focus:border-sky-500"
              >
                <option value="all">All Time</option>
                <option value="15m">Last 15 mins</option>
                <option value="1h">Last 1 hour</option>
                <option value="24h">Last 24 hours</option>
              </select>
            </div>
          </div>
        </div>

        <div className="text-xs text-slate-400 font-mono">
          Showing {filteredEvents.length} events (Live buffer: {events.length})
        </div>
      </div>

      {/* Stream Table */}
      <div className="rounded-xl border border-slate-800 bg-slate-900/60 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left text-xs">
            <thead className="bg-slate-900/90 text-slate-400 border-b border-slate-800 text-[11px] font-mono uppercase tracking-wider">
              <tr>
                <th className="py-3 px-4">Time</th>
                <th className="py-3 px-4">Job ID</th>
                <th className="py-3 px-4">Queue</th>
                <th className="py-3 px-4">Task Name</th>
                <th className="py-3 px-4">Transition</th>
                <th className="py-3 px-4">Duration</th>
                <th className="py-3 px-4 text-right">Action</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {filteredEvents.length === 0 ? (
                <tr>
                  <td colSpan={7} className="py-12 text-center text-slate-500">
                    No transition events match the current filter criteria
                  </td>
                </tr>
              ) : (
                filteredEvents.map((ev, i) => (
                  <tr key={`${ev.id}-${ev.toState}-${i}`} className="hover:bg-slate-800/40 transition-colors">
                    <td className="py-2.5 px-4 font-mono text-slate-400 whitespace-nowrap">
                      {new Date(ev.timestamp).toLocaleTimeString()}
                    </td>
                    <td className="py-2.5 px-4 font-mono text-slate-200 whitespace-nowrap">
                      <span className="bg-slate-800/70 px-1.5 py-0.5 rounded border border-slate-700/60 text-sky-400">
                        {ev.id}
                      </span>
                    </td>
                    <td className="py-2.5 px-4 whitespace-nowrap">
                      <span className="font-medium text-slate-300">{ev.queue}</span>
                      <span className="ml-1 text-[10px] text-slate-500 font-mono">({ev.queueType})</span>
                    </td>
                    <td className="py-2.5 px-4 font-mono text-slate-200 max-w-[200px] truncate" title={ev.name}>
                      {ev.name}
                    </td>
                    <td className="py-2.5 px-4 whitespace-nowrap">
                      <div className="flex items-center gap-1.5">
                        {ev.fromState && (
                          <>
                            <span className="text-slate-500 text-[10px] uppercase font-mono">{ev.fromState}</span>
                            <ArrowRight className="h-3 w-3 text-slate-600" />
                          </>
                        )}
                        {getStateBadge(ev.toState)}
                      </div>
                    </td>
                    <td className="py-2.5 px-4 font-mono text-slate-400 whitespace-nowrap">
                      {ev.durationMs ? `${ev.durationMs}ms` : ev.waitTimeMs ? `wait: ${ev.waitTimeMs}ms` : '-'}
                    </td>
                    <td className="py-2.5 px-4 text-right whitespace-nowrap">
                      <button
                        onClick={() => onInspectJob(ev.queueType, ev.queue, ev.id)}
                        className="inline-flex items-center gap-1 px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-sky-400 hover:text-sky-300 transition-colors"
                      >
                        <Eye className="h-3 w-3" />
                        Inspect
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
};
