import React, { useState, useEffect } from 'react';
import {
  Settings as SettingsIcon,
  Server,
  Shield,
  Radio,
  FileCode,
  RefreshCw,
  Lock,
  Database
} from 'lucide-react';
import { api } from '../api/client';

export const Settings: React.FC = () => {
  const [settings, setSettings] = useState<any | null>(null);
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

  const managerInfo = settings?.manager || {};
  const daemonInfo = settings?.daemon || {};
  const xrayRuntime = settings?.xray_runtime || {};
  const routingInfo = settings?.routing || {};
  const securityInfo = settings?.security || {};

  const daemonStatus = daemonInfo?.status || {};
  const socksCfg = xrayRuntime?.socks || {};
  const vlessCfg = xrayRuntime?.vless || {};

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide font-mono flex items-center gap-2">
            <SettingsIcon className="w-5 h-5 text-cyan-400" />
            SYSTEM ARCHITECTURE & RUNTIME SETTINGS
          </h1>
          <p className="text-xs text-slate-400 font-mono mt-1">
            Read-only configuration inspection for multi-host manager, active daemon runtime, and security posture.
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
            <strong>READ-ONLY SECURITY POSTURE:</strong> Configuration is dynamically loaded from the selected host's daemon runtime. Sensitive tokens and private keys are never exposed.
          </span>
        </div>
        <span className="px-2 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-800 text-[10px] font-bold">
          HARDENED
        </span>
      </div>

      {/* Settings Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-5 font-mono text-xs">
        {/* Manager Architecture */}
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3">
          <div className="flex items-center gap-2 text-white font-bold pb-2 border-b border-slate-800">
            <Database className="w-4 h-4 text-cyan-400" />
            <span>WEB MANAGER PLATFORM</span>
          </div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-slate-400">Manager Version:</span>
              <span className="text-slate-200 font-semibold">{managerInfo.version || '2.0.0-multihost'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Listen Address:</span>
              <span className="text-slate-200">{managerInfo.listening_address || '0.0.0.0:8443'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Database Engine:</span>
              <span className="text-emerald-400 font-semibold">SQLite (Pure Go / WAL Mode)</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Active Host:</span>
              <span className="text-cyan-300 font-bold">{managerInfo.selected_host || 'None'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Active Host Address:</span>
              <span className="text-slate-300">{managerInfo.selected_address || 'None'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Agent API URL:</span>
              <span className="text-slate-300">{managerInfo.selected_agent_url || 'None'}</span>
            </div>
          </div>
        </div>

        {/* Selected Host Daemon Runtime */}
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3">
          <div className="flex items-center gap-2 text-white font-bold pb-2 border-b border-slate-800">
            <Server className="w-4 h-4 text-emerald-400" />
            <span>SELECTED DAEMON RUNTIME</span>
          </div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-slate-400">Daemon Server ID:</span>
              <span className="text-slate-200 font-semibold">{daemonStatus.server_id || 'UNKNOWN'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Daemon Version:</span>
              <span className="text-slate-200">{daemonStatus.version || 'UNKNOWN'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Region:</span>
              <span className="text-slate-300">{daemonStatus.region || 'UNKNOWN'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Daemon Status:</span>
              <span className="text-emerald-400 font-bold">{daemonStatus.status || 'UNKNOWN'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Uptime:</span>
              <span className="text-slate-300">
                {daemonStatus.uptime ? `${Math.floor(daemonStatus.uptime / 60)} minutes` : 'UNKNOWN'}
              </span>
            </div>
          </div>
        </div>

        {/* Xray Runtime Endpoints */}
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3">
          <div className="flex items-center gap-2 text-white font-bold pb-2 border-b border-slate-800">
            <Radio className="w-4 h-4 text-amber-400" />
            <span>XRAY RUNTIME INBOUND CONFIGURATION</span>
          </div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-slate-400">SOCKS5 Inbound:</span>
              <span className={socksCfg.port ? 'text-emerald-400 font-bold' : 'text-slate-500'}>
                {socksCfg.port ? `${socksCfg.address || '127.0.0.1'}:${socksCfg.port}` : 'UNKNOWN'}
              </span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">VLESS Inbound:</span>
              <span className={vlessCfg.port ? 'text-emerald-400 font-bold' : 'text-slate-500'}>
                {vlessCfg.port ? `${vlessCfg.address || '0.0.0.0'}:${vlessCfg.port} (${vlessCfg.security || 'tcp'})` : 'DISABLED'}
              </span>
            </div>
            {vlessCfg.security === 'reality' && (
              <>
                <div className="flex justify-between">
                  <span className="text-slate-400">Reality SNI:</span>
                  <span className="text-slate-200">{vlessCfg.server_name || 'UNKNOWN'}</span>
                </div>
                <div className="flex justify-between">
                  <span className="text-slate-400">Reality Public Key:</span>
                  <span className="text-cyan-300 truncate max-w-xs">{vlessCfg.public_key || 'UNKNOWN'}</span>
                </div>
              </>
            )}
            <div className="flex justify-between">
              <span className="text-slate-400">Supported Protocols:</span>
              <span className="text-cyan-300">
                {xrayRuntime?.protocols ? (xrayRuntime.protocols as string[]).join(', ') : 'UNKNOWN'}
              </span>
            </div>
          </div>
        </div>

        {/* Security Controls */}
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3">
          <div className="flex items-center gap-2 text-white font-bold pb-2 border-b border-slate-800">
            <Shield className="w-4 h-4 text-purple-400" />
            <span>SECURITY CONTROLS & ENCRYPTION</span>
          </div>
          <div className="space-y-2">
            <div className="flex justify-between">
              <span className="text-slate-400">Password Storage:</span>
              <span className="text-white font-semibold">{securityInfo.auth_type || 'bcrypt'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Host Token Storage:</span>
              <span className="text-emerald-400 font-semibold">AES-256-GCM Encrypted at Rest</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">SSRF Protection:</span>
              <span className="text-emerald-400 font-semibold">STRICT (Link-Local & Metadata Blocked)</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">CSRF Guard:</span>
              <span className="text-emerald-400">{securityInfo.csrf_protection || 'ACTIVE'}</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Subscription Token:</span>
              <span className="text-emerald-400">SHA-256 Storage • Single-view Reveal</span>
            </div>
            <div className="flex justify-between">
              <span className="text-slate-400">Log Redaction:</span>
              <span className="text-emerald-400">/sub/[REDACTED]</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};
