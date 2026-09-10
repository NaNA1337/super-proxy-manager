import React from 'react';
import { Shield, Server, Activity, LogOut, User as UserIcon, RefreshCw } from 'lucide-react';
import { UserSession, DaemonStatus, SystemStats } from '../types';

interface NavbarProps {
  session: UserSession;
  status: DaemonStatus | null;
  system: SystemStats | null;
  onLogout: () => void;
  onRefresh: () => void;
  isRefreshing: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({
  session,
  status,
  system,
  onLogout,
  onRefresh,
  isRefreshing,
}) => {
  const formatUptime = (seconds: number) => {
    if (!seconds) return '0s';
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    if (d > 0) return `${d}d ${h}h ${m}m`;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m ${seconds % 60}s`;
  };

  return (
    <header className="sticky top-0 z-40 h-16 glass-panel border-b border-slate-800/80 px-6 flex items-center justify-between">
      {/* Brand & Daemon Status */}
      <div className="flex items-center gap-6">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-cyan-500/10 border border-cyan-500/40 text-cyan-400 glow-cyan">
            <Shield className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-bold text-white tracking-wide text-base">SUPER-PROXY</span>
              <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-700/50 font-bold">
                NOC v{status?.version || '1.0.0'}
              </span>
            </div>
            <div className="flex items-center gap-2 text-xs text-slate-400">
              <span className="h-1.5 w-1.5 rounded-full bg-emerald-400 animate-pulse" />
              <span className="font-mono">{status?.name || 'Egress Manager'}</span>
              <span className="text-slate-600">|</span>
              <span className="font-mono text-cyan-300">Host: {status?.server_id || 'localhost'}</span>
            </div>
          </div>
        </div>

        {/* Telemetry quick indicators */}
        <div className="hidden lg:flex items-center gap-4 pl-4 border-l border-slate-800">
          <div className="flex items-center gap-2 font-mono text-xs">
            <Server className="w-4 h-4 text-slate-500" />
            <span className="text-slate-400">CPU:</span>
            <span className="font-bold text-slate-200">
              {system ? `${system.CPU.toFixed(1)}%` : '--'}
            </span>
          </div>
          <div className="flex items-center gap-2 font-mono text-xs">
            <Activity className="w-4 h-4 text-slate-500" />
            <span className="text-slate-400">RAM:</span>
            <span className="font-bold text-slate-200">
              {system ? `${system.Memory.toFixed(1)}%` : '--'}
            </span>
          </div>
          <div className="flex items-center gap-2 font-mono text-xs">
            <span className="text-slate-400">Uptime:</span>
            <span className="text-emerald-400 font-bold">
              {status ? formatUptime(status.uptime) : '--'}
            </span>
          </div>
        </div>
      </div>

      {/* Right controls: Refresh, User pill, Logout */}
      <div className="flex items-center gap-3">
        <button
          onClick={onRefresh}
          disabled={isRefreshing}
          title="Refresh Data"
          className="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800/80 transition active:scale-95 disabled:opacity-50"
        >
          <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin text-cyan-400' : ''}`} />
        </button>

        {/* User Pill */}
        <div className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-800">
          <UserIcon className="w-4 h-4 text-slate-400" />
          <span className="text-xs font-medium text-slate-200">{session.username || 'user'}</span>
          <span
            className={`text-[10px] uppercase font-mono px-1.5 py-0.5 rounded font-bold ${
              session.role === 'admin'
                ? 'bg-rose-950 text-rose-300 border border-rose-800/50'
                : 'bg-slate-800 text-slate-300'
            }`}
          >
            {session.role || 'readonly'}
          </span>
        </div>

        {/* Logout */}
        <button
          onClick={onLogout}
          className="flex items-center gap-1.5 px-3 py-1.5 text-xs text-rose-400 hover:text-rose-300 hover:bg-rose-950/30 rounded-lg border border-transparent hover:border-rose-800/40 transition"
        >
          <LogOut className="w-4 h-4" />
          <span className="hidden sm:inline">Logout</span>
        </button>
      </div>
    </header>
  );
};
