import React, { useState, useRef, useEffect } from 'react';
import {
  Shield,
  Server,
  Activity,
  LogOut,
  User as UserIcon,
  RefreshCw,
  ChevronDown,
  Plus,
  Radio,
  Globe
} from 'lucide-react';
import { UserSession, DaemonStatus, SystemStats, Host } from '../types';

interface NavbarProps {
  session: UserSession;
  status: DaemonStatus | null;
  system: SystemStats | null;
  hosts: Host[];
  selectedHostID: string;
  onSelectHost: (hostID: string) => void;
  onNavigateToHosts: () => void;
  onLogout: () => void;
  onRefresh: () => void;
  isRefreshing: boolean;
}

export const Navbar: React.FC<NavbarProps> = ({
  session,
  status,
  system,
  hosts,
  selectedHostID,
  onSelectHost,
  onNavigateToHosts,
  onLogout,
  onRefresh,
  isRefreshing,
}) => {
  const [hostDropdownOpen, setHostDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setHostDropdownOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  const formatUptime = (seconds: number) => {
    if (!seconds) return '0s';
    const d = Math.floor(seconds / 86400);
    const h = Math.floor((seconds % 86400) / 3600);
    const m = Math.floor((seconds % 3600) / 60);
    if (d > 0) return `${d}d ${h}h ${m}m`;
    if (h > 0) return `${h}h ${m}m`;
    return `${m}m ${seconds % 60}s`;
  };

  const currentHost = hosts.find((h) => h.id === selectedHostID);
  const isAllHosts = selectedHostID === 'all';

  return (
    <header className="sticky top-0 z-40 h-16 glass-panel border-b border-slate-800/80 px-4 sm:px-6 flex items-center justify-between">
      {/* Left: Brand & Host Selector */}
      <div className="flex items-center gap-4 lg:gap-6">
        <div className="flex items-center gap-3">
          <div className="p-2 rounded-lg bg-cyan-500/10 border border-cyan-500/40 text-cyan-400 glow-cyan">
            <Shield className="w-5 h-5" />
          </div>
          <div>
            <div className="flex items-center gap-2">
              <span className="font-bold text-white tracking-wide text-base font-mono">SUPER-PROXY</span>
              <span className="text-[10px] uppercase font-mono px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-700/50 font-bold">
                NOC v2.0
              </span>
            </div>
            <div className="text-[11px] text-slate-400 font-mono hidden sm:block">
              Multi-Host Control Panel
            </div>
          </div>
        </div>

        {/* Host Selector Dropdown */}
        <div className="relative pl-2 sm:pl-4 border-l border-slate-800" ref={dropdownRef}>
          <button
            onClick={() => setHostDropdownOpen(!hostDropdownOpen)}
            className="flex items-center gap-2 px-3 py-1.5 rounded-lg bg-slate-900 border border-slate-700/70 hover:border-cyan-500/60 font-mono text-xs text-slate-200 transition"
          >
            <Server className="w-3.5 h-3.5 text-cyan-400" />
            <span className="font-bold text-cyan-300">
              HOST: {isAllHosts ? 'All Hosts' : currentHost ? currentHost.name : hosts.length === 0 ? 'No Hosts' : 'Select Host'}
            </span>
            <ChevronDown className="w-3.5 h-3.5 text-slate-400" />
          </button>

          {hostDropdownOpen && (
            <div className="absolute left-2 sm:left-4 top-11 w-64 glass-panel rounded-xl border border-slate-700 shadow-2xl p-2 z-50 font-mono text-xs space-y-1">
              <div className="px-2 py-1 text-[10px] uppercase text-slate-500 font-bold tracking-wider">
                Configured Hosts ({hosts.length})
              </div>

              {hosts.map((h) => (
                <button
                  key={h.id}
                  onClick={() => {
                    onSelectHost(h.id);
                    setHostDropdownOpen(false);
                  }}
                  className={`w-full flex items-center justify-between px-2.5 py-2 rounded-lg hover:bg-slate-800 transition text-left ${
                    selectedHostID === h.id ? 'bg-cyan-950/40 text-cyan-300 font-bold border border-cyan-800/40' : 'text-slate-300'
                  }`}
                >
                  <div className="flex items-center gap-2 truncate">
                    <span
                      className={`w-1.5 h-1.5 rounded-full ${
                        h.status === 'healthy' ? 'bg-emerald-400' : 'bg-rose-400'
                      }`}
                    />
                    <span className="truncate">{h.name}</span>
                  </div>
                  <span className="text-[10px] text-slate-500 uppercase">{h.region || 'global'}</span>
                </button>
              ))}

              {hosts.length > 1 && (
                <button
                  onClick={() => {
                    onSelectHost('all');
                    setHostDropdownOpen(false);
                  }}
                  className={`w-full flex items-center gap-2 px-2.5 py-2 rounded-lg hover:bg-slate-800 transition text-left ${
                    isAllHosts ? 'bg-cyan-950/40 text-cyan-300 font-bold border border-cyan-800/40' : 'text-slate-300'
                  }`}
                >
                  <Globe className="w-3.5 h-3.5 text-cyan-400" />
                  <span>All Hosts (Aggregated)</span>
                </button>
              )}

              <div className="pt-1.5 mt-1 border-t border-slate-800">
                <button
                  onClick={() => {
                    setHostDropdownOpen(false);
                    onNavigateToHosts();
                  }}
                  className="w-full flex items-center justify-center gap-1.5 px-2.5 py-1.5 rounded-lg bg-cyan-600/20 hover:bg-cyan-600/30 text-cyan-300 transition text-[11px] font-bold"
                >
                  <Plus className="w-3 h-3" />
                  <span>Manage / Add Hosts</span>
                </button>
              </div>
            </div>
          )}
        </div>

        {/* Telemetry quick indicators */}
        <div className="hidden xl:flex items-center gap-4 pl-4 border-l border-slate-800">
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
      <div className="flex items-center gap-2 sm:gap-3">
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
