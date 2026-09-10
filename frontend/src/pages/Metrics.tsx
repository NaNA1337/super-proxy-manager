import React, { useState, useEffect } from 'react';
import {
  BarChart3,
  Activity,
  Zap,
  Radio,
  RefreshCw,
  AlertTriangle,
  Server,
  Layers,
  ArrowUpRight,
  ShieldCheck
} from 'lucide-react';
import { api } from '../api/client';

export const Metrics: React.FC = () => {
  const [rawMetrics, setRawMetrics] = useState<string>('');
  const [parsed, setParsed] = useState<Record<string, number>>({});
  const [loading, setLoading] = useState(true);

  const fetchMetrics = async () => {
    try {
      const text = await api.getMetricsText();
      setRawMetrics(text);

      // Simple, robust Prometheus text parser
      const map: Record<string, number> = {};
      const lines = text.split('\n');
      for (const line of lines) {
        const trimmed = line.trim();
        if (!trimmed || trimmed.startsWith('#')) continue;
        const parts = trimmed.split(/\s+/);
        if (parts.length >= 2) {
          const val = parseFloat(parts[parts.length - 1]);
          if (!isNaN(val)) {
            // strip labels for metric key overview
            const name = parts[0].split('{')[0];
            map[name] = val;
          }
        }
      }
      setParsed(map);
    } catch (e) {
      console.error('Failed to load prometheus metrics', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchMetrics();
    const interval = setInterval(fetchMetrics, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide">PROMETHEUS TELEMETRY</h1>
          <p className="text-xs text-slate-400">
            Real-time scraped metrics directly from daemon control plane Prometheus endpoint
          </p>
        </div>
        <button
          onClick={fetchMetrics}
          className="flex items-center gap-1.5 px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 font-mono text-xs rounded-lg transition"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-cyan-400' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Metrics Cards Grid */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-4 font-mono">
        {/* Active Slots */}
        <div className="glass-panel p-4 rounded-xl border border-slate-800">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>active_slots</span>
            <Layers className="w-4 h-4 text-cyan-400" />
          </div>
          <div className="text-2xl font-bold text-white mt-2">
            {parsed['active_slots'] ?? '--'}
          </div>
          <span className="text-[11px] text-slate-500">Current online egress slots</span>
        </div>

        {/* Standby Slots */}
        <div className="glass-panel p-4 rounded-xl border border-slate-800">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>standby_slots</span>
            <Radio className="w-4 h-4 text-teal-400" />
          </div>
          <div className="text-2xl font-bold text-teal-300 mt-2">
            {parsed['standby_slots'] ?? '--'}
          </div>
          <span className="text-[11px] text-slate-500">Warm backup tunnels ready</span>
        </div>

        {/* Draining Slots */}
        <div className="glass-panel p-4 rounded-xl border border-slate-800">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>draining_slots</span>
            <Activity className="w-4 h-4 text-amber-400" />
          </div>
          <div className="text-2xl font-bold text-amber-400 mt-2">
            {parsed['draining_slots'] ?? 0}
          </div>
          <span className="text-[11px] text-slate-500">Graceful connection drains</span>
        </div>

        {/* Tunnel UP */}
        <div className="glass-panel p-4 rounded-xl border border-slate-800">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>tunnel_up</span>
            <ArrowUpRight className="w-4 h-4 text-emerald-400" />
          </div>
          <div className="text-2xl font-bold text-emerald-400 mt-2">
            {parsed['tunnel_up'] ?? '--'}
          </div>
          <span className="text-[11px] text-slate-500">Cumulative tunnel connects</span>
        </div>

        {/* Tunnel DOWN */}
        <div className="glass-panel p-4 rounded-xl border border-slate-800">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>tunnel_down</span>
            <AlertTriangle className="w-4 h-4 text-rose-400" />
          </div>
          <div className="text-2xl font-bold text-rose-400 mt-2">
            {parsed['tunnel_down'] ?? 0}
          </div>
          <span className="text-[11px] text-slate-500">Disconnects / failovers</span>
        </div>

        {/* Xray Restarts */}
        <div className="glass-panel p-4 rounded-xl border border-slate-800">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>xray_restarts</span>
            <Server className="w-4 h-4 text-purple-400" />
          </div>
          <div className="text-2xl font-bold text-purple-400 mt-2">
            {parsed['xray_restarts'] ?? 0}
          </div>
          <span className="text-[11px] text-slate-500">Supervisor process restarts</span>
        </div>

        {/* Node Failures */}
        <div className="glass-panel p-4 rounded-xl border border-slate-800">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>node_failures</span>
            <AlertTriangle className="w-4 h-4 text-amber-400" />
          </div>
          <div className="text-2xl font-bold text-slate-200 mt-2">
            {parsed['node_failures'] ?? 0}
          </div>
          <span className="text-[11px] text-slate-500">Failed verification events</span>
        </div>

        {/* Dead Nodes */}
        <div className="glass-panel p-4 rounded-xl border border-slate-800">
          <div className="flex items-center justify-between text-xs text-slate-400">
            <span>node_dead</span>
            <AlertTriangle className="w-4 h-4 text-red-500" />
          </div>
          <div className="text-2xl font-bold text-red-400 mt-2">
            {parsed['node_dead'] ?? 0}
          </div>
          <span className="text-[11px] text-slate-500">Exceeded max retry threshold</span>
        </div>
      </div>

      {/* Raw Prometheus Exporter Stream */}
      <div className="glass-panel p-5 rounded-xl border border-slate-800 font-mono text-xs space-y-2">
        <div className="flex items-center justify-between pb-2 border-b border-slate-800 text-slate-300">
          <div className="flex items-center gap-2 font-bold">
            <BarChart3 className="w-4 h-4 text-cyan-400" />
            <span>RAW PROMETHEUS METRIC STREAM (/metrics)</span>
          </div>
          <span className="text-[11px] text-slate-500">Type: text/plain; version=0.0.4</span>
        </div>
        <pre className="p-4 bg-noc-950 rounded-lg border border-slate-800/80 text-slate-300 overflow-x-auto max-h-80 overflow-y-auto leading-relaxed text-[11px]">
          {rawMetrics || 'Fetching metrics...'}
        </pre>
      </div>
    </div>
  );
};
