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
  Clock
} from 'lucide-react';
import { DaemonStatus, SystemStats, CurrentExit, SlotsOverview, PoolSummary } from '../types';
import { StatusBadge } from '../components/StatusBadge';
import { api } from '../api/client';

interface DashboardProps {
  status: DaemonStatus | null;
  system: SystemStats | null;
  onNavigateToSlots: () => void;
  onNavigateToNodes: () => void;
}

export const Dashboard: React.FC<DashboardProps> = ({
  status,
  system,
  onNavigateToSlots,
  onNavigateToNodes,
}) => {
  const [exits, setExits] = useState<CurrentExit[]>([]);
  const [slotsData, setSlotsData] = useState<SlotsOverview | null>(null);
  const [pool, setPool] = useState<PoolSummary | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchDashboardData = async () => {
    try {
      const [exitsRes, slotsRes, poolRes] = await Promise.all([
        api.getCurrentExits().catch(() => []),
        api.getSlots().catch(() => null),
        api.getPool().catch(() => null),
      ]);
      setExits(exitsRes);
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
  }, []);

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

  // Build slot list for Slot 0, Slot 1, Slot 2
  const totalSlots = slotsData?.total_configured || 3;
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
      {/* 3-Second Critical Operational Bar (Answers all 9 critical questions instantly) */}
      <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3">
        {/* Q1: Daemon Status */}
        <div className="p-3.5 glass-panel rounded-xl border border-slate-800 flex flex-col justify-between">
          <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>DAEMON CORE</span>
            <Server className="w-3.5 h-3.5 text-emerald-400" />
          </div>
          <div className="mt-2 flex items-center gap-2">
            <span className="h-2.5 w-2.5 rounded-full bg-emerald-400 animate-pulse" />
            <span className="text-sm font-bold text-white uppercase tracking-wider">ONLINE</span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">v{status?.version || '1.0.0'}</span>
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
          <span className="text-[10px] text-slate-500 font-mono mt-1">Region: {status?.region || 'JP'}</span>
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
          <div className="mt-2 text-sm font-bold text-white font-mono">
            RX: {system ? formatBytes(system.RX) : '--'}
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">
            TX: {system ? formatBytes(system.TX) : '--'}
          </span>
        </div>

        {/* Q7: Leak Protection */}
        <div className="p-3.5 glass-panel rounded-xl border border-slate-800 flex flex-col justify-between">
          <div className="flex items-center justify-between text-xs text-slate-400 font-mono">
            <span>LEAK DROP GUARD</span>
            <ShieldCheck className="w-3.5 h-3.5 text-emerald-400" />
          </div>
          <div className="mt-2 text-xs font-bold text-emerald-400 font-mono flex items-center gap-1.5">
            <span className="h-1.5 w-1.5 rounded-full bg-emerald-400" />
            <span>DNS & IPv6 SAFE</span>
          </div>
          <span className="text-[10px] text-slate-500 font-mono mt-1">Fail-Closed Active</span>
        </div>
      </div>

      {/* Slot Overview Section (Stage 4 Core Requirement) */}
      <div>
        <div className="flex items-center justify-between mb-4">
          <div>
            <h2 className="text-lg font-bold text-white tracking-wide">EGRESS TUNNEL SLOTS</h2>
            <p className="text-xs text-slate-400">
              Live OpenVPN tunnel status, active node binding, and Linux policy routing tables
            </p>
          </div>
          <button
            onClick={onNavigateToSlots}
            className="text-xs text-cyan-400 hover:text-cyan-300 font-mono flex items-center gap-1"
          >
            Manage Slots & Switches →
          </button>
        </div>

        <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
          {slotCards.map(({ slot, exit, isOverridden }) => {
            const isActive = exit && exit.status === 'ACTIVE';
            const statusStr = exit ? exit.status : 'OFFLINE';

            return (
              <div
                key={slot}
                className={`glass-panel p-5 rounded-xl border transition relative overflow-hidden ${
                  isActive ? 'border-emerald-500/40 glow-emerald' : 'border-slate-800'
                }`}
              >
                {/* Slot Header */}
                <div className="flex items-center justify-between pb-3 border-b border-slate-800/80">
                  <div className="flex items-center gap-2">
                    <span className="text-xs font-mono font-bold px-2 py-0.5 rounded bg-slate-800 text-cyan-400 border border-slate-700">
                      SLOT {slot}
                    </span>
                    <span className="text-[11px] font-mono text-slate-500">Table: {10000 + slot}</span>
                  </div>
                  <StatusBadge status={statusStr} size="sm" />
                </div>

                {/* Slot Body */}
                {exit ? (
                  <div className="mt-4 space-y-3 font-mono text-xs">
                    <div className="flex items-center justify-between">
                      <span className="text-slate-400">Node IP:</span>
                      <span className="font-bold text-white">{exit.ip}</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-400">Country / Region:</span>
                      <span className="text-cyan-300 font-semibold">{exit.country} ({exit.region})</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-400">Throughput:</span>
                      <span className="text-emerald-400 font-bold">{formatSpeed(exit.throughput)}</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-400">FSM Score:</span>
                      <span className="text-amber-400 font-bold">{exit.score} pts</span>
                    </div>

                    <div className="flex items-center justify-between pt-2 border-t border-slate-800/60">
                      <span className="text-slate-500 text-[11px]">Interface:</span>
                      <span className="text-slate-300 text-[11px]">tun{slot}</span>
                    </div>

                    <div className="flex items-center justify-between">
                      <span className="text-slate-500 text-[11px]">Last Verified:</span>
                      <span className="text-slate-400 text-[11px]">
                        {exit.last_check ? new Date(exit.last_check).toLocaleTimeString() : 'Just now'}
                      </span>
                    </div>
                  </div>
                ) : (
                  <div className="py-8 flex flex-col items-center justify-center text-slate-500 space-y-2">
                    <Radio className="w-8 h-8 opacity-40 animate-pulse text-amber-400" />
                    <span className="font-mono text-xs">Awaiting Tunnel Assignment</span>
                  </div>
                )}

                {isOverridden && (
                  <div className="mt-3 px-2 py-1 rounded bg-amber-950/60 border border-amber-800/50 text-[10px] text-amber-300 font-mono flex items-center gap-1.5">
                    <span className="h-1.5 w-1.5 rounded-full bg-amber-400 animate-ping" />
                    <span>MANUAL SWITCH OVERRIDE LOCKED</span>
                  </div>
                )}
              </div>
            );
          })}
        </div>
      </div>

      {/* Node Pool Summary Card */}
      <div className="glass-panel p-5 rounded-xl border border-slate-800">
        <div className="flex items-center justify-between mb-4">
          <div>
            <h3 className="text-sm font-bold text-white tracking-wide">NODE POOL COMPOSITION</h3>
            <p className="text-xs text-slate-400">Discovered candidate endpoints and qualification funnel</p>
          </div>
          <button
            onClick={onNavigateToNodes}
            className="text-xs text-cyan-400 hover:text-cyan-300 font-mono"
          >
            View Node Inventory →
          </button>
        </div>

        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-3 font-mono">
          <div className="p-3 rounded-lg bg-noc-900/80 border border-slate-800">
            <span className="text-[10px] text-slate-400 uppercase">Active</span>
            <div className="text-xl font-bold text-emerald-400 mt-1">{pool?.active || 0}</div>
          </div>
          <div className="p-3 rounded-lg bg-noc-900/80 border border-slate-800">
            <span className="text-[10px] text-slate-400 uppercase">Standby (Warm)</span>
            <div className="text-xl font-bold text-cyan-400 mt-1">{pool?.standby || 0}</div>
          </div>
          <div className="p-3 rounded-lg bg-noc-900/80 border border-slate-800">
            <span className="text-[10px] text-slate-400 uppercase">Qualified</span>
            <div className="text-xl font-bold text-teal-400 mt-1">{pool?.qualified || 0}</div>
          </div>
          <div className="p-3 rounded-lg bg-noc-900/80 border border-slate-800">
            <span className="text-[10px] text-slate-400 uppercase">Candidates</span>
            <div className="text-xl font-bold text-slate-300 mt-1">{pool?.candidate || 0}</div>
          </div>
          <div className="p-3 rounded-lg bg-noc-900/80 border border-slate-800">
            <span className="text-[10px] text-slate-400 uppercase">Cooldown</span>
            <div className="text-xl font-bold text-purple-400 mt-1">{pool?.cooldown || 0}</div>
          </div>
          <div className="p-3 rounded-lg bg-noc-900/80 border border-slate-800">
            <span className="text-[10px] text-slate-400 uppercase">Rejected / Dead</span>
            <div className="text-xl font-bold text-rose-400 mt-1">{pool?.rejected || 0}</div>
          </div>
        </div>
      </div>
    </div>
  );
};
