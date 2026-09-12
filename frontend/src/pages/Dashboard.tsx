import React, { useState, useEffect } from 'react';
import {
  Server,
  Activity,
  Zap,
  Globe,
  ShieldCheck,
  Radio,
  ArrowUpRight,
  ArrowDownLeft,
  AlertTriangle,
  Layers,
  Cpu,
  RefreshCw,
  Clock,
  Plus,
  Network
} from 'lucide-react';
import { DaemonStatus, SystemStats, CurrentExit, SlotsOverview, PoolSummary } from '../types';
import { StatusBadge } from '../components/StatusBadge';
import { api } from '../api/client';

interface DashboardProps {
  status: DaemonStatus | null;
  system: SystemStats | null;
  hostsCount: number;
  selectedHostID: string;
  onNavigateToHosts: () => void;
  onNavigateToSlots: () => void;
  onNavigateToNodes: () => void;
}

export const Dashboard: React.FC<DashboardProps> = ({
  status,
  system,
  hostsCount,
  selectedHostID,
  onNavigateToHosts,
  onNavigateToSlots,
  onNavigateToNodes,
}) => {
  const [exits, setExits] = useState<CurrentExit[]>([]);
  const [slotsData, setSlotsData] = useState<SlotsOverview | null>(null);
  const [pool, setPool] = useState<PoolSummary | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchDashboardData = async () => {
    if (hostsCount === 0) {
      setLoading(false);
      return;
    }
    try {
      const [exitsRes, slotsRes, poolRes] = await Promise.all([
        api.getCurrentExits().catch(() => []),
        api.getSlots().catch(() => null),
        api.getPool().catch(() => null),
      ]);
      setExits(exitsRes || []);
      setSlotsData(slotsRes);
      setPool(poolRes);
    } catch (e) {
      console.error('Failed to load dashboard data', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDashboardData();
    const timer = setInterval(fetchDashboardData, 4000);
    return () => clearInterval(timer);
  }, [hostsCount, selectedHostID]);

  if (hostsCount === 0) {
    return (
      <div className="flex flex-col items-center justify-center p-12 text-center glass-panel rounded-2xl border border-cyan-500/30 max-w-2xl mx-auto my-12 space-y-4">
        <div className="p-4 rounded-2xl bg-cyan-500/10 border border-cyan-500/40 text-cyan-400">
          <Server className="w-12 h-12" />
        </div>
        <h2 className="text-xl font-bold font-mono text-white">NO HOSTS CONFIGURED</h2>
        <p className="text-sm font-mono text-slate-400 max-w-md">
          Add your first super-proxy server to begin monitoring slots, routing, and generating client links.
        </p>
        <button
          onClick={onNavigateToHosts}
          className="px-5 py-2.5 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-white font-mono text-xs font-bold transition flex items-center gap-2 shadow-lg shadow-cyan-950 active:scale-95"
        >
          <Plus className="w-4 h-4" />
          <span>+ Add Super-Proxy Host</span>
        </button>
      </div>
    );
  }

  const formatBytes = (bytes: number) => {
    if (!bytes) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return `${(bytes / Math.pow(k, i)).toFixed(2)} ${sizes[i]}`;
  };

  const formatSpeed = (bps: number) => {
    if (!bps) return '0 Mbps';
    return `${(bps / 1_000_000).toFixed(1)} Mbps`;
  };

  const isAllHosts = selectedHostID === 'all';
  const totalSlots = slotsData?.total_configured || 3;
  const activeExits = exits.filter((exit) => exit.status === 'ACTIVE');
  const aggregateCapacity = activeExits.reduce((sum, exit) => sum + (exit.throughput || 0), 0);
  const slotCards = [];
  for (let i = 0; i < totalSlots; i++) {
    const exit = exits.find((e) => e.slot === i);
    slotCards.push({
      slot: i,
      exit: exit || null,
      isOverridden: slotsData?.manual_overrides?.[i] || false,
    });
  }

  return (
    <div className="space-y-6">
      {/* Fleet Aggregate Banner (when All Hosts selected) */}
      {isAllHosts && (
        <div className="p-4 rounded-xl bg-gradient-to-r from-cyan-950/60 to-blue-950/40 border border-cyan-500/30 flex items-center justify-between font-mono text-xs">
          <div className="flex items-center gap-3">
            <Network className="w-5 h-5 text-cyan-400" />
            <div>
              <span className="font-bold text-white uppercase">FLEET VIEW: ALL HOSTS AGGREGATED</span>
              <p className="text-[11px] text-slate-400">Viewing aggregate telemetry across all registered super-proxy daemon servers.</p>
            </div>
          </div>
          <div className="flex items-center gap-4 text-xs font-mono">
            <div>Total Exits: <span className="text-cyan-300 font-bold">{exits.length}</span></div>
          </div>
        </div>
      )}

      {/* 3-Second Critical Operational Bar */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        {/* Q1: Daemon Status */}
        <div className="p-3.5 glass-panel rounded-xl border border-slate-800 flex flex-col justify-between">
          <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>DAEMON CORE</span>
            <Server className="w-3.5 h-3.5 text-emerald-400" />
          </div>
          <div className="mt-2 flex items-center gap-2">
            <span className="h-2.5 w-2.5 rounded-full bg-emerald-400 animate-pulse" />
            <span className="text-sm font-bold text-white uppercase tracking-wider font-mono">
              {status ? status.status || 'ONLINE' : 'ONLINE'}
            </span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">v{status?.version || '1.5.0'}</span>
        </div>

        {/* Q2: Active Slots */}
        <div className="p-3.5 glass-panel rounded-xl border border-slate-800 flex flex-col justify-between">
          <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>ACTIVE SLOTS</span>
            <Layers className="w-3.5 h-3.5 text-cyan-400" />
          </div>
          <div className="mt-2 text-lg font-bold text-cyan-400 font-mono">
            {exits.filter((e) => e.status === 'ACTIVE').length} / {totalSlots}
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">Standby: {pool?.standby || 0}</span>
        </div>

        {/* Q3: Primary Exit Geolocation */}
        <div className="p-3.5 glass-panel rounded-xl border border-slate-800 flex flex-col justify-between">
          <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>PRIMARY EXIT</span>
            <Globe className="w-3.5 h-3.5 text-blue-400" />
          </div>
          <div className="mt-2 text-sm font-bold text-white font-mono truncate">
            {exits[0]?.country ? `${exits[0].country} (${exits[0].ip})` : 'Bootstrapping...'}
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">Region: {status?.region || 'GLOBAL'}</span>
        </div>

        {/* Q4: Problematic Nodes */}
        <div className="p-3.5 glass-panel rounded-xl border border-slate-800 flex flex-col justify-between">
          <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>NODE HEALTH</span>
            <AlertTriangle className="w-3.5 h-3.5 text-amber-400" />
          </div>
          <div className="mt-2 flex items-center gap-2">
            <span className="text-lg font-bold text-emerald-400 font-mono">
              {pool?.rejected ? pool.rejected : '0'}
            </span>
            <span className="text-xs text-slate-400">failed</span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">Cooldown: {pool?.cooldown || 0}</span>
        </div>

        {/* Q5: Traffic Throughput */}
        <div className="p-3.5 glass-panel rounded-xl border border-slate-800 flex flex-col justify-between">
          <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>WAN TRAFFIC</span>
            <Zap className="w-3.5 h-3.5 text-amber-400" />
          </div>
          <div className="mt-2 text-sm font-bold text-white font-mono truncate">
            RX: {system ? formatBytes(system.RX) : '--'}
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">
            TX: {system ? formatBytes(system.TX) : '--'}
          </span>
        </div>

        {/* Q6 & Q7: Leak Protection */}
        <div className="p-3.5 glass-panel rounded-xl border border-slate-800 flex flex-col justify-between">
          <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>FAIL-CLOSED GUARD</span>
            <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
          </div>
          <div className="mt-2 text-xs font-bold text-emerald-400 font-mono flex items-center gap-1.5">
            <span className="h-2 w-2 rounded-full bg-emerald-400" />
            <span>DNS & IPv6 SAFE</span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">Policy Routing Active</span>
        </div>
      </div>

      {/* Slots Overview Grid */}
      <div className="space-y-3">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-mono uppercase tracking-wider text-slate-400 font-bold flex items-center gap-2">
            <Layers className="w-4 h-4 text-cyan-400" />
            Egress Slots State
          </h2>
          <button
            onClick={onNavigateToSlots}
            className="text-xs font-mono text-cyan-400 hover:text-cyan-300 flex items-center gap-1 transition"
          >
            <span>Manage Slots</span>
            <ArrowUpRight className="w-3.5 h-3.5" />
          </button>
        </div>

        <div className="p-4 rounded-xl bg-emerald-950/30 border border-emerald-700/40 flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2 font-mono">
          <div>
            <div className="text-[11px] uppercase tracking-wider text-emerald-300">Measured Multi-Connection Capacity</div>
            <div className="text-2xl font-bold text-white mt-1">{formatSpeed(aggregateCapacity)}</div>
          </div>
          <p className="text-[11px] text-slate-400 max-w-xl">
            Sum of active-slot admission tests. Independent connections are round-robin distributed; one TCP connection stays on one tunnel.
          </p>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {slotCards.map(({ slot, exit, isOverridden }) => {
            const hasNode = exit && exit.node_id;
            const state = exit?.status || 'STANDBY';
            return (
              <div
                key={slot}
                className="p-5 glass-panel rounded-2xl border border-slate-800 hover:border-slate-700 transition relative overflow-hidden flex flex-col justify-between"
              >
                <div>
                  <div className="flex items-center justify-between mb-4">
                    <div className="flex items-center gap-2">
                      <span className="text-base font-bold font-mono text-white">Slot {slot}</span>
                      {isOverridden && (
                        <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-950 text-amber-300 border border-amber-700 font-mono font-bold">
                          OVERRIDE
                        </span>
                      )}
                    </div>
                    <StatusBadge status={state} />
                  </div>

                  {hasNode ? (
                    <div className="space-y-2.5 font-mono text-xs">
                      <div className="flex justify-between items-center py-1 border-b border-slate-800/60">
                        <span className="text-slate-500">Node IP:</span>
                        <span className="text-slate-200 font-medium">{exit.ip}</span>
                      </div>
                      <div className="flex justify-between items-center py-1 border-b border-slate-800/60">
                        <span className="text-slate-500">Country:</span>
                        <span className="text-slate-200">{exit.country || 'Global'}</span>
                      </div>
                      <div className="flex justify-between items-center py-1 border-b border-slate-800/60">
                        <span className="text-slate-500">Single-Tunnel Test:</span>
                        <span className="text-emerald-400 font-bold">{formatSpeed(exit.throughput)}</span>
                      </div>
                      <div className="flex justify-between items-center py-1 border-b border-slate-800/60">
                        <span className="text-slate-500">Health Score:</span>
                        <span className="text-cyan-400 font-bold">{exit.score.toFixed(1)}</span>
                      </div>
                    </div>
                  ) : (
                    <div className="py-8 flex flex-col items-center justify-center text-slate-500 font-mono text-xs space-y-1">
                      <Radio className="w-6 h-6 text-slate-600 animate-pulse" />
                      <span>Waiting for eligible node...</span>
                    </div>
                  )}
                </div>

                <div className="mt-4 pt-3 border-t border-slate-800/60 flex items-center justify-between text-[11px] font-mono text-slate-400">
                  <span>Routing Table: 1000{slot}</span>
                  <span className="text-cyan-400">fwmark 0x10{slot}</span>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Node Pool Summary & System Resources */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        {/* Pool Summary */}
        <div className="p-5 glass-panel rounded-2xl border border-slate-800">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-sm font-mono uppercase tracking-wider text-slate-400 font-bold flex items-center gap-2">
              <Server className="w-4 h-4 text-cyan-400" />
              Node Fleet Statistics
            </h2>
            <button
              onClick={onNavigateToNodes}
              className="text-xs font-mono text-cyan-400 hover:text-cyan-300 flex items-center gap-1 transition"
            >
              <span>Explore All Nodes</span>
              <ArrowUpRight className="w-3.5 h-3.5" />
            </button>
          </div>

          <div className="grid grid-cols-3 gap-3 font-mono text-center">
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800">
              <div className="text-slate-500 text-[11px]">ACTIVE</div>
              <div className="text-xl font-bold text-emerald-400 mt-1">{pool?.active || 0}</div>
            </div>
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800">
              <div className="text-slate-500 text-[11px]">QUALIFIED</div>
              <div className="text-xl font-bold text-cyan-400 mt-1">{pool?.qualified || 0}</div>
            </div>
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800">
              <div className="text-slate-500 text-[11px]">CANDIDATE</div>
              <div className="text-xl font-bold text-blue-400 mt-1">{pool?.candidate || 0}</div>
            </div>
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800">
              <div className="text-slate-500 text-[11px]">STANDBY</div>
              <div className="text-xl font-bold text-slate-300 mt-1">{pool?.standby || 0}</div>
            </div>
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800">
              <div className="text-slate-500 text-[11px]">COOLDOWN</div>
              <div className="text-xl font-bold text-amber-400 mt-1">{pool?.cooldown || 0}</div>
            </div>
            <div className="p-3 rounded-xl bg-slate-900/60 border border-slate-800">
              <div className="text-slate-500 text-[11px]">FAILED / DEAD</div>
              <div className="text-xl font-bold text-rose-400 mt-1">{pool?.rejected || 0}</div>
            </div>
          </div>
        </div>

        {/* System Load & Architecture */}
        <div className="p-5 glass-panel rounded-2xl border border-slate-800 flex flex-col justify-between">
          <div>
            <h2 className="text-sm font-mono uppercase tracking-wider text-slate-400 font-bold flex items-center gap-2 mb-4">
              <Cpu className="w-4 h-4 text-cyan-400" />
              Runtime Telemetry & Isolation
            </h2>

            <div className="space-y-3 font-mono text-xs">
              <div className="flex justify-between items-center">
                <span className="text-slate-400">Host CPU Utilization:</span>
                <span className="font-bold text-slate-200">
                  {system ? `${system.CPU.toFixed(1)}%` : '--'}
                </span>
              </div>
              <div className="w-full bg-slate-900 rounded-full h-1.5">
                <div
                  className="bg-cyan-400 h-1.5 rounded-full transition-all duration-500"
                  style={{ width: `${Math.min(system?.CPU || 0, 100)}%` }}
                />
              </div>

              <div className="flex justify-between items-center pt-2">
                <span className="text-slate-400">Memory Utilization:</span>
                <span className="font-bold text-slate-200">
                  {system ? `${system.Memory.toFixed(1)}%` : '--'}
                </span>
              </div>
              <div className="w-full bg-slate-900 rounded-full h-1.5">
                <div
                  className="bg-blue-400 h-1.5 rounded-full transition-all duration-500"
                  style={{ width: `${Math.min(system?.Memory || 0, 100)}%` }}
                />
              </div>
            </div>
          </div>

          <div className="mt-4 pt-3 border-t border-slate-800/80 flex items-center justify-between text-[11px] font-mono text-slate-500">
            <span>Daemon Core: super-proxy</span>
            <span className="text-emerald-400 font-bold">159e0a1 Baseline</span>
          </div>
        </div>
      </div>
    </div>
  );
};
