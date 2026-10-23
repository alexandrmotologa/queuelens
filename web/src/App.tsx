import React, { useState, useEffect, useCallback } from 'react';
import { Header } from './components/Header';
import { QueueOverviewGrid } from './components/QueueOverviewGrid';
import { JobStreamTable } from './components/JobStreamTable';
import { DlqClusterView } from './components/DlqClusterView';
import { StallMonitor } from './components/StallMonitor';
import { PayloadModal } from './components/PayloadModal';
import { useQueueStream } from './hooks/useQueueStream';
import { QueueInfo, ServerStatus, ErrorCluster, StallWarning, Job, QueueType } from './types';

export function App() {
  const [activeTab, setActiveTab] = useState<'overview' | 'stream' | 'dlq' | 'stalls'>('overview');
  const [status, setStatus] = useState<ServerStatus | null>(null);
  const [queues, setQueues] = useState<QueueInfo[]>([]);
  const [clusters, setClusters] = useState<ErrorCluster[]>([]);
  const [stalls, setStalls] = useState<StallWarning[]>([]);
  const [selectedJob, setSelectedJob] = useState<Job | null>(null);
  const [replayingClusterId, setReplayingClusterId] = useState<string | null>(null);

  const { events, metrics, connected: wsConnected } = useQueueStream();

  const fetchAllData = useCallback(async () => {
    try {
      const [statusRes, queuesRes, dlqRes, stallsRes] = await Promise.all([
        fetch('/api/status').then((r) => r.json()),
        fetch('/api/queues').then((r) => r.json()),
        fetch('/api/dlq/clusters').then((r) => r.json()),
        fetch('/api/stalls').then((r) => r.json()),
      ]);

      setStatus(statusRes);
      setQueues(queuesRes.queues || []);
      setClusters(dlqRes.clusters || []);
      setStalls(stallsRes.stalls || []);
    } catch (e) {
      console.error('Failed to fetch data', e);
    }
  }, []);

  useEffect(() => {
    fetchAllData();
    const interval = setInterval(fetchAllData, 8000);
    return () => clearInterval(interval);
  }, [fetchAllData]);

  const handleInspectJob = async (queueType: string, queue: string, id: string) => {
    try {
      const res = await fetch(`/api/jobs/${queueType}/${queue}/${id}`);
      if (res.ok) {
        const job = await res.json();
        setSelectedJob(job);
      } else {
        alert(`Job ${id} not found or already processed`);
      }
    } catch (e) {
      console.error('Failed to fetch job', e);
    }
  };

  const handleRetryJob = async (job: Job, mutatedPayload: string) => {
    try {
      const res = await fetch(`/api/jobs/${job.queueType}/${job.queue}/${job.id}/retry`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ payload: mutatedPayload }),
      });
      if (res.ok) {
        fetchAllData();
      } else {
        const err = await res.json();
        alert(`Failed to retry: ${err.error}`);
      }
    } catch (e) {
      console.error('Error retrying job', e);
    }
  };

  const handleDeleteJob = async (job: Job) => {
    try {
      const res = await fetch(`/api/jobs/${job.queueType}/${job.queue}/${job.id}`, {
        method: 'DELETE',
      });
      if (res.ok) {
        fetchAllData();
      }
    } catch (e) {
      console.error('Error deleting job', e);
    }
  };

  const handleReplayCluster = async (cluster: ErrorCluster, rateLimit: number) => {
    setReplayingClusterId(cluster.id);
    try {
      const res = await fetch('/api/dlq/replay', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          queueType: cluster.sampleQueueType,
          queue: cluster.sampleQueue,
          jobIds: cluster.jobIds,
          rateLimitPerSec: rateLimit,
        }),
      });
      if (res.ok) {
        await fetchAllData();
      } else {
        const err = await res.json();
        alert(`Failed to mass replay cluster: ${err.error}`);
      }
    } catch (e) {
      console.error('Replay error', e);
    } finally {
      setReplayingClusterId(null);
    }
  };

  const handleForceUnlock = async (queueType: QueueType, queue: string, lockKey: string) => {
    try {
      const res = await fetch('/api/stalls/unlock', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ queueType, queue, lockKey }),
      });
      if (res.ok) {
        fetchAllData();
      }
    } catch (e) {
      console.error('Unlock error', e);
    }
  };

  const handlePurgeQueue = async (qType: QueueType, queue: string, state: string) => {
    if (!window.confirm(`Purge all ${state} jobs from queue "${queue}"? This action cannot be undone.`)) {
      return;
    }
    try {
      const res = await fetch(`/api/queues/${qType}/${queue}/purge?state=${state}`, {
        method: 'POST',
      });
      if (res.ok) {
        fetchAllData();
      }
    } catch (e) {
      console.error('Purge error', e);
    }
  };

  const totalDlqJobs = clusters.reduce((acc, c) => acc + c.count, 0);

  return (
    <div className="min-h-screen bg-[#090d16] text-slate-100 flex flex-col font-sans">
      <Header
        status={status}
        activeTab={activeTab}
        setActiveTab={setActiveTab}
        wsConnected={wsConnected}
        onRefresh={fetchAllData}
        stallCount={stalls.length}
        dlqCount={totalDlqJobs}
      />

      <main className="flex-1 max-w-7xl w-full mx-auto p-6">
        {activeTab === 'overview' && (
          <QueueOverviewGrid
            queues={queues}
            metrics={metrics}
            onSelectQueue={(_name) => setActiveTab('stream')}
            onPurgeQueue={handlePurgeQueue}
            stallCount={stalls.length}
          />
        )}

        {activeTab === 'stream' && (
          <JobStreamTable
            events={events}
            queues={queues}
            onInspectJob={handleInspectJob}
          />
        )}

        {activeTab === 'dlq' && (
          <DlqClusterView
            clusters={clusters}
            onReplayCluster={handleReplayCluster}
            onInspectJob={handleInspectJob}
            replayingClusterId={replayingClusterId}
          />
        )}

        {activeTab === 'stalls' && (
          <StallMonitor
            stalls={stalls}
            onForceUnlock={handleForceUnlock}
            onInspectJob={handleInspectJob}
          />
        )}
      </main>

      <PayloadModal
        job={selectedJob}
        onClose={() => setSelectedJob(null)}
        onRetry={handleRetryJob}
        onDelete={handleDeleteJob}
      />
    </div>
  );
}
