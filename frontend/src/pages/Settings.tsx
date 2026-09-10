import React, { useState, useEffect } from 'react';
import {
  Settings as SettingsIcon,
  Server,
  Shield,
  Radio,
  FileCode,
  RefreshCw,
  Lock
} from 'lucide-react';
import { api } from '../api/client';

export const Settings: React.FC = () => {
  const [settings, setSettings] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchSettings = async () => {
    try {
      const data = await api.getSettings();
      setSettings(data);
    } catch (e) {
      console.error('Failed to load settings', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSettings();
  }, []);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide">SYSTEM ARCHITECTURE CONFIGURATION</h1>
          <p className="text-xs text-slate-400">
            Read-only configuration inspection for daemon core, Xray inbounds/outbounds, and policy routing rules
          </p>
        </div>
        <button
          onClick={fetchSettings}
          className="flex items-center gap-1.5 px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 font-mono text-xs rounded-lg transition"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-cyan-400' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Notice Banner */}
      <div className="p-4 rounded-xl glass-panel border border-cyan-500/40 font-mono text-xs text-slate-300 flex items-center justify-between">
        <div className="flex items-center gap-3">
          <Lock className="w-5 h-5 text-cyan-400 shrink-0" />
          <span>
            <strong>READ-ONLY SECURITY POSTURE:</strong> Core network daemon settings are locked to prevent inadvertent connection disruption or routing instability.
          </span>
        </div>
        <span className="px-2 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-800 text-[10px] font-bold">
          IMMUTABLE
        </span>
      </div>

      {/* Settings Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-5 font-mono text-xs">
        {/* Core Daemon Config */}
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3">
          <div className="flex items-center gap-2 text-white font-bold pb-2 border-b border-slate-800">
            <Server className="w-4 h-4 text-cyan-400" />
            <span>SUPER-PROXY DAEMON RUNTIME</span>
          </div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-slate-400">Daemon Architecture:</span>
              <span className="text-slate-200 font-semibold">Decoupled Thin Web / Go Daemon</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Agent Control Plane:</span>
              <span className="text-emerald-400 font-semibold">https://127.0.0.1:60000 (TLS)</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Max Active Slots:</span>
              <span className="text-cyan-300">3 Dedicated Tunnels</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Standby Capacity:</span>
              <span className="text-cyan-300">2 Warm Backup Slots</span>
            </div>
          </div>
        </div>

        {/* Xray Configuration */}
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3">
          <div className="flex items-center gap-2 text-white font-bold pb-2 border-b border-slate-800">
            <Radio className="w-4 h-4 text-amber-400" />
            <span>XRAY CORE ROUTING RUNTIME</span>
          </div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-slate-400">Xray Binary:</span>
              <span className="text-slate-200">/usr/local/bin/xray</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Internal API:</span>
              <span className="text-slate-200">127.0.0.1:10085 (dokodemo-door)</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">SOCKS Inbound:</span>
              <span className="text-emerald-400">127.0.0.1:1080 (Active)</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Balancer Tag:</span>
              <span className="text-slate-200">vpn-balancer (exit-0, exit-1, exit-2)</span>
            </div>
          </div>
        </div>

        {/* Policy Routing */}
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3">
          <div className="flex items-center gap-2 text-white font-bold pb-2 border-b border-slate-800">
            <FileCode className="w-4 h-4 text-emerald-400" />
            <span>LINUX POLICY ROUTING TABLES</span>
          </div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-slate-400">Base Routing Table ID:</span>
              <span className="text-white font-bold">10000</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Base Draining Table ID:</span>
              <span className="text-white font-bold">20000</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">DNS Drop Chain:</span>
              <span className="text-emerald-400">iptables DROP --dport 53</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">IPv6 Drop Chain:</span>
              <span className="text-emerald-400">ip6tables DROP non-loopback</span>
            </div>
          </div>
        </div>

        {/* Security Parameters */}
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3">
          <div className="flex items-center gap-2 text-white font-bold pb-2 border-b border-slate-800">
            <Shield className="w-4 h-4 text-purple-400" />
            <span>SECURITY CONTROLS & PROTECTION</span>
          </div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-slate-400">Session Cookies:</span>
              <span className="text-white">HttpOnly • SameSite=Strict • Secure</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Password Encryption:</span>
              <span className="text-white">bcrypt (Default Cost 10)</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">CSRF Guard:</span>
              <span className="text-emerald-400">Constant-time token validation</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Subscription Secrets:</span>
              <span className="text-emerald-400">SHA-256 Hashed • Zero Raw Tokens</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
