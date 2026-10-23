import React, { useState, useEffect } from 'react';
import { X, Bell, Send, Check, AlertTriangle, ShieldCheck } from 'lucide-react';

interface AlertsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const AlertsModal: React.FC<AlertsModalProps> = ({ isOpen, onClose }) => {
  if (!isOpen) return null;

  const [webhookUrl, setWebhookUrl] = useState('');
  const [saving, setSaving] = useState(false);
  const [testing, setTesting] = useState(false);
  const [statusMsg, setStatusMsg] = useState<{ text: string; error?: boolean } | null>(null);

  useEffect(() => {
    fetch('/api/alerts/config')
      .then((r) => r.json())
      .then((data) => {
        if (data.webhookUrl) setWebhookUrl(data.webhookUrl);
      })
      .catch((err) => console.error(err));
  }, []);

  const handleSave = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setStatusMsg(null);
    try {
      const res = await fetch('/api/alerts/config', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ webhookUrl }),
      });
      if (res.ok) {
        setStatusMsg({ text: 'Webhook URL updated successfully' });
      } else {
        const err = await res.json();
        setStatusMsg({ text: err.error || 'Failed to save', error: true });
      }
    } catch (e: any) {
      setStatusMsg({ text: e.message, error: true });
    } finally {
      setSaving(false);
    }
  };

  const handleTestAlert = async () => {
    setTesting(true);
    setStatusMsg(null);
    try {
      const res = await fetch('/api/alerts/test', { method: 'POST' });
      if (res.ok) {
        setStatusMsg({ text: 'Test alert dispatched! Check your Slack or Discord channel.' });
      } else {
        const err = await res.json();
        setStatusMsg({ text: err.error || 'Test alert failed', error: true });
      }
    } catch (e: any) {
      setStatusMsg({ text: e.message, error: true });
    } finally {
      setTesting(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/70 backdrop-blur-sm">
      <div className="w-full max-w-lg rounded-2xl bg-slate-900 border border-slate-800 shadow-2xl overflow-hidden flex flex-col">
        {/* Header */}
        <div className="p-4 border-b border-slate-800 flex items-center justify-between bg-slate-950/60">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-amber-500/10 text-amber-400 border border-amber-500/20">
              <Bell className="h-5 w-5" />
            </div>
            <div>
              <h3 className="font-semibold text-slate-100 text-sm">Operational Alerting & Webhooks</h3>
              <p className="text-xs text-slate-400">Receive instant alerts for dead-worker stalls & error rate spikes</p>
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
        <form onSubmit={handleSave} className="p-5 space-y-4">
          <div>
            <label className="text-xs font-semibold text-slate-300 block mb-1.5">
              Webhook Endpoint (Slack, Discord, PagerDuty, or Generic HTTP)
            </label>
            <input
              type="url"
              value={webhookUrl}
              onChange={(e) => setWebhookUrl(e.target.value)}
              placeholder="https://hooks.slack.com/services/... or https://discord.com/api/webhooks/..."
              className="w-full font-mono text-xs px-3 py-2 rounded-xl bg-slate-950 border border-slate-800 text-slate-100 placeholder-slate-600 focus:outline-none focus:border-amber-500 focus:ring-1 focus:ring-amber-500 transition-all"
            />
          </div>

          <div className="p-3.5 rounded-xl bg-slate-950/60 border border-slate-800/80 space-y-2 text-xs text-slate-400">
            <div className="font-medium text-slate-300 flex items-center gap-1.5">
              <ShieldCheck className="h-4 w-4 text-emerald-400" />
              Automated Alert Triggers:
            </div>
            <ul className="list-disc list-inside space-y-1 text-[11px] text-slate-400">
              <li><strong className="text-slate-300">Stall Warning:</strong> Active task held by an expired worker lock.</li>
              <li><strong className="text-slate-300">Failure Spike:</strong> Error rate breaches 10% threshold.</li>
              <li><strong className="text-slate-300">SLA Degradation:</strong> Backlog drain time exceeds 30 minutes.</li>
            </ul>
          </div>

          {statusMsg && (
            <div
              className={`p-3 rounded-lg text-xs font-mono ${
                statusMsg.error
                  ? 'bg-red-950/40 border border-red-900/50 text-red-400'
                  : 'bg-emerald-950/40 border border-emerald-900/50 text-emerald-400'
              }`}
            >
              {statusMsg.text}
            </div>
          )}

          {/* Footer Actions */}
          <div className="pt-2 flex items-center justify-between border-t border-slate-800">
            <button
              type="button"
              disabled={testing || !webhookUrl}
              onClick={handleTestAlert}
              className="flex items-center gap-1.5 px-3 py-1.5 rounded-lg text-xs bg-slate-800 hover:bg-slate-700 text-slate-300 disabled:opacity-50 transition-colors"
            >
              <Send className="h-3 w-3" />
              {testing ? 'Testing...' : 'Test Webhook'}
            </button>

            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={onClose}
                className="px-3.5 py-1.5 rounded-lg text-xs text-slate-400 hover:text-slate-200 hover:bg-slate-800 transition-colors"
              >
                Close
              </button>
              <button
                type="submit"
                disabled={saving}
                className="flex items-center gap-1.5 px-4 py-1.5 rounded-lg text-xs font-semibold bg-amber-500 hover:bg-amber-400 text-slate-950 font-bold transition-all disabled:opacity-50 shadow-sm shadow-amber-500/20"
              >
                <Check className="h-3.5 w-3.5" />
                {saving ? 'Saving...' : 'Save Configuration'}
              </button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
};
