import React, { useState, useEffect } from 'react';
import {
  Server,
  Plus,
  Activity,
  CheckCircle2,
  XCircle,
  AlertCircle,
  RefreshCw,
  Trash2,
  Edit2,
  Star,
  ExternalLink,
  ShieldCheck,
  Radio
} from 'lucide-react';
import { Host, TestConnectionResult } from '../types';
import { api, setSelectedHostID, getSelectedHostID } from '../api/client';

interface HostsProps {
  isAdmin: boolean;
  onHostSelected?: (hostID: string) => void;
}

export const Hosts: React.FC<HostsProps> = ({ isAdmin, onHostSelected }) => {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Modal State
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [wizardStep, setWizardStep] = useState<1 | 2 | 3>(1);
  const [formName, setFormName] = useState('');
  const [formAddress, setFormAddress] = useState('');
  const [formAgentURL, setFormAgentURL] = useState('https://127.0.0.1:60000');
  const [formToken, setFormToken] = useState('');
  const [formIsDefault, setFormIsDefault] = useState(false);
  const [testResult, setTestResult] = useState<TestConnectionResult | null>(null);
  const [testing, setTesting] = useState(false);
  const [saving, setSaving] = useState(false);

  // Action State
  const [probingHostID, setProbingHostID] = useState<string | null>(null);
  const [activeHostID, setActiveHostID] = useState<string>(getSelectedHostID());

  const fetchHosts = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listHosts();
      setHosts(data || []);
      if (!activeHostID && data && data.length > 0) {
        const def = data.find((h) => h.is_default) || data[0];
        setActiveHostID(def.id);
        setSelectedHostID(def.id);
      }
    } catch (err: any) {
      setError(err.safeMessage || err.message || 'Failed to load hosts');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchHosts();
  }, []);

  const handleTestConnection = async () => {
    setTesting(true);
    setTestResult(null);
    try {
      const res = await api.testHostPreSave({
        agent_url: formAgentURL,
        token: formToken,
      });
      setTestResult(res);
    } catch (err: any) {
      setTestResult({
        success: false,
        status: 'offline',
        error: err.safeMessage || err.message || 'Connection test failed',
      });
    } finally {
      setTesting(false);
    }
  };

  const handleSaveHost = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const created = await api.createHost({
        name: formName,
        address: formAddress,
        agent_url: formAgentURL,
        token: formToken,
        is_default: formIsDefault,
      });
      setIsModalOpen(false);
      resetModal();
      await fetchHosts();
      if (formIsDefault || hosts.length === 0) {
        setActiveHostID(created.id);
        setSelectedHostID(created.id);
        if (onHostSelected) onHostSelected(created.id);
      }
    } catch (err: any) {
      setError(err.safeMessage || err.message || 'Failed to create host');
    } finally {
      setSaving(false);
    }
  };

  const resetModal = () => {
    setWizardStep(1);
    setFormName('');
    setFormAddress('');
    setFormAgentURL('https://127.0.0.1:60000');
    setFormToken('');
    setFormIsDefault(false);
    setTestResult(null);
  };

  const handleProbe = async (hostID: string) => {
    setProbingHostID(hostID);
    try {
      await api.testHost(hostID);
      await fetchHosts();
    } catch (err: any) {
      alert(`Probe failed: ${err.safeMessage || err.message}`);
    } finally {
      setProbingHostID(null);
    }
  };

  const handleSetDefault = async (hostID: string) => {
    try {
      await api.setDefaultHost(hostID);
      await fetchHosts();
    } catch (err: any) {
      alert(`Failed to set default: ${err.safeMessage || err.message}`);
    }
  };

  const handleDelete = async (hostID: string, name: string) => {
    if (!confirm(`Are you sure you want to delete host "${name}"?`)) return;
    try {
      await api.deleteHost(hostID);
      await fetchHosts();
      if (activeHostID === hostID) {
        setSelectedHostID('');
        setActiveHostID('');
      }
    } catch (err: any) {
      alert(`Delete failed: ${err.safeMessage || err.message}`);
    }
  };

  const handleSelectHost = (hostID: string) => {
    setActiveHostID(hostID);
    setSelectedHostID(hostID);
    if (onHostSelected) onHostSelected(hostID);
  };

  return (
    <div className="space-y-6">
      {/* Header & Actions */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white font-mono flex items-center gap-2">
            <Server className="w-5 h-5 text-cyan-400" />
            SUPER-PROXY HOSTS
          </h1>
          <p className="text-xs text-slate-400 font-mono mt-1">
            Manage distributed super-proxy daemon servers across data centers.
          </p>
        </div>

        <div className="flex items-center gap-2.5">
          <button
            onClick={fetchHosts}
            className="px-3 py-2 rounded-lg bg-slate-900 border border-slate-800 text-xs font-mono text-slate-300 hover:text-white transition flex items-center gap-1.5"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-cyan-400' : ''}`} />
            Refresh
          </button>
          {isAdmin && (
            <button
              onClick={() => setIsModalOpen(true)}
              className="px-3.5 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-xs font-mono font-medium text-white transition flex items-center gap-1.5 shadow-lg shadow-cyan-950 active:scale-95"
            >
              <Plus className="w-3.5 h-3.5" />
              Add Host
            </button>
          )}
        </div>
      </div>

      {error && (
        <div className="p-3.5 rounded-xl bg-rose-950/60 border border-rose-800/80 text-rose-300 text-xs flex items-center gap-2">
          <AlertCircle className="w-4 h-4 shrink-0" />
          <span>{error}</span>
        </div>
      )}

      {/* Hosts Table */}
      <div className="glass-panel rounded-2xl border border-slate-800 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left border-collapse font-mono text-xs">
            <thead>
              <tr className="border-b border-slate-800 bg-slate-900/60 text-slate-400 uppercase text-[11px] tracking-wider">
                <th className="py-3 px-4">Active</th>
                <th className="py-3 px-4">Host Name</th>
                <th className="py-3 px-4">Address / Agent URL</th>
                <th className="py-3 px-4">Status</th>
                <th className="py-3 px-4">Region</th>
                <th className="py-3 px-4">Version</th>
                <th className="py-3 px-4">Latency</th>
                <th className="py-3 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {hosts.length === 0 ? (
                <tr>
                  <td colSpan={8} className="py-12 text-center text-slate-500">
                    {loading ? (
                      <div className="flex items-center justify-center gap-2">
                        <div className="w-4 h-4 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin" />
                        <span>Loading hosts...</span>
                      </div>
                    ) : (
                      <div className="space-y-3">
                        <p>No super-proxy daemon hosts configured.</p>
                        {isAdmin && (
                          <button
                            onClick={() => setIsModalOpen(true)}
                            className="px-3.5 py-1.5 rounded-lg bg-cyan-600/20 border border-cyan-500/40 text-cyan-300 text-xs hover:bg-cyan-600/30 transition"
                          >
                            + Add Your First Host
                          </button>
                        )}
                      </div>
                    )}
                  </td>
                </tr>
              ) : (
                hosts.map((h) => {
                  const isSelected = activeHostID === h.id;
                  return (
                    <tr
                      key={h.id}
                      className={`hover:bg-slate-800/40 transition cursor-pointer ${
                        isSelected ? 'bg-cyan-950/20' : ''
                      }`}
                      onClick={() => handleSelectHost(h.id)}
                    >
                      <td className="py-3 px-4">
                        <Radio className={`w-4 h-4 ${isSelected ? 'text-cyan-400 fill-cyan-400/20' : 'text-slate-600'}`} />
                      </td>
                      <td className="py-3 px-4">
                        <div className="flex items-center gap-2">
                          <span className="font-bold text-slate-200">{h.name}</span>
                          {h.is_default && (
                            <span className="text-[10px] px-1.5 py-0.5 rounded bg-amber-950/80 text-amber-300 border border-amber-800/50 flex items-center gap-1 font-bold">
                              <Star className="w-2.5 h-2.5 fill-amber-400" />
                              DEFAULT
                            </span>
                          )}
                        </div>
                      </td>
                      <td className="py-3 px-4 text-slate-400">
                        <div className="font-medium text-slate-300">{h.address}</div>
                        <div className="text-[11px] text-slate-500">{h.agent_url}</div>
                      </td>
                      <td className="py-3 px-4">
                        <span
                          className={`inline-flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold ${
                            h.status === 'healthy'
                              ? 'bg-emerald-950 text-emerald-300 border border-emerald-800/60'
                              : h.status === 'degraded'
                              ? 'bg-amber-950 text-amber-300 border border-amber-800/60'
                              : 'bg-rose-950 text-rose-300 border border-rose-800/60'
                          }`}
                        >
                          <span
                            className={`w-1.5 h-1.5 rounded-full ${
                              h.status === 'healthy' ? 'bg-emerald-400' : 'bg-rose-400'
                            }`}
                          />
                          {h.status.toUpperCase()}
                        </span>
                      </td>
                      <td className="py-3 px-4 text-slate-300">{h.region || '--'}</td>
                      <td className="py-3 px-4 text-slate-300">{h.version || '--'}</td>
                      <td className="py-3 px-4 text-slate-400">
                        {h.latency_ms ? `${h.latency_ms}ms` : '--'}
                      </td>
                      <td className="py-3 px-4 text-right" onClick={(e) => e.stopPropagation()}>
                        <div className="flex items-center justify-end gap-1.5">
                          <button
                            onClick={() => handleProbe(h.id)}
                            disabled={probingHostID === h.id}
                            title="Test / Probe Connection"
                            className="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-cyan-400 transition"
                          >
                            <RefreshCw className={`w-3.5 h-3.5 ${probingHostID === h.id ? 'animate-spin text-cyan-400' : ''}`} />
                          </button>
                          {isAdmin && !h.is_default && (
                            <button
                              onClick={() => handleSetDefault(h.id)}
                              title="Set as Default Host"
                              className="p-1.5 rounded hover:bg-slate-800 text-slate-400 hover:text-amber-400 transition"
                            >
                              <Star className="w-3.5 h-3.5" />
                            </button>
                          )}
                          {isAdmin && (
                            <button
                              onClick={() => handleDelete(h.id, h.name)}
                              title="Delete Host"
                              className="p-1.5 rounded hover:bg-rose-950/40 text-slate-400 hover:text-rose-400 transition"
                            >
                              <Trash2 className="w-3.5 h-3.5" />
                            </button>
                          )}
                        </div>
                      </td>
                    </tr>
                  );
                })
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Add Host Wizard Modal */}
      {isModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm">
          <div className="glass-panel w-full max-w-lg rounded-2xl border border-cyan-500/30 p-6 shadow-2xl">
            <div className="flex items-center justify-between pb-4 border-b border-slate-800">
              <div className="flex items-center gap-2">
                <Server className="w-5 h-5 text-cyan-400" />
                <h2 className="text-base font-bold text-white font-mono">
                  ADD SUPER-PROXY HOST
                </h2>
              </div>
              <button
                onClick={() => {
                  setIsModalOpen(false);
                  resetModal();
                }}
                className="text-slate-400 hover:text-white"
              >
                <XCircle className="w-5 h-5" />
              </button>
            </div>

            {/* Steps Indicator */}
            <div className="flex items-center justify-between py-4 border-b border-slate-800/80 font-mono text-xs">
              <span className={wizardStep === 1 ? 'text-cyan-400 font-bold' : 'text-slate-500'}>
                1. Host Details
              </span>
              <span className="text-slate-700">→</span>
              <span className={wizardStep === 2 ? 'text-cyan-400 font-bold' : 'text-slate-500'}>
                2. Agent API & Token
              </span>
              <span className="text-slate-700">→</span>
              <span className={wizardStep === 3 ? 'text-cyan-400 font-bold' : 'text-slate-500'}>
                3. Test & Confirm
              </span>
            </div>

            <form onSubmit={handleSaveHost} className="space-y-4 pt-4">
              {wizardStep === 1 && (
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs font-mono uppercase text-slate-400 mb-1">
                      Host Name
                    </label>
                    <input
                      type="text"
                      required
                      placeholder="e.g. Tokyo-01, US-West-02"
                      value={formName}
                      onChange={(e) => setFormName(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-slate-100 placeholder-slate-500 font-mono focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                  <div>
                    <label className="block text-xs font-mono uppercase text-slate-400 mb-1">
                      Address / Domain / Public IP
                    </label>
                    <input
                      type="text"
                      required
                      placeholder="e.g. jp01.example.com or 203.0.113.1"
                      value={formAddress}
                      onChange={(e) => setFormAddress(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-slate-100 placeholder-slate-500 font-mono focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                  <div className="pt-2 flex justify-end">
                    <button
                      type="button"
                      disabled={!formName || !formAddress}
                      onClick={() => setWizardStep(2)}
                      className="px-4 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-xs font-mono text-white disabled:opacity-50 transition"
                    >
                      Next: API Credentials →
                    </button>
                  </div>
                </div>
              )}

              {wizardStep === 2 && (
                <div className="space-y-3">
                  <div>
                    <label className="block text-xs font-mono uppercase text-slate-400 mb-1">
                      Agent API URL (HTTPS)
                    </label>
                    <input
                      type="url"
                      required
                      placeholder="https://127.0.0.1:60000 or https://node.example.com:60000"
                      value={formAgentURL}
                      onChange={(e) => setFormAgentURL(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-slate-100 placeholder-slate-500 font-mono focus:outline-none focus:border-cyan-500"
                    />
                    <p className="text-[10px] text-slate-500 font-mono mt-1">
                      Protected with SSRF guards. Supports TLS control plane with bearer token.
                    </p>
                  </div>
                  <div>
                    <label className="block text-xs font-mono uppercase text-slate-400 mb-1">
                      Agent API Token
                    </label>
                    <input
                      type="password"
                      placeholder="Bearer token configured on super-proxy daemon"
                      value={formToken}
                      onChange={(e) => setFormToken(e.target.value)}
                      className="w-full px-3 py-2 bg-slate-900 border border-slate-700 rounded-lg text-sm text-slate-100 placeholder-slate-500 font-mono focus:outline-none focus:border-cyan-500"
                    />
                    <p className="text-[10px] text-slate-500 font-mono mt-1">
                      Encrypted with AES-256-GCM at rest. Never exposed to browser or logs.
                    </p>
                  </div>
                  <div className="pt-2 flex justify-between">
                    <button
                      type="button"
                      onClick={() => setWizardStep(1)}
                      className="px-3 py-2 rounded-lg bg-slate-800 text-xs font-mono text-slate-300 hover:text-white"
                    >
                      ← Back
                    </button>
                    <button
                      type="button"
                      disabled={!formAgentURL}
                      onClick={() => {
                        setWizardStep(3);
                        handleTestConnection();
                      }}
                      className="px-4 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-xs font-mono text-white disabled:opacity-50 transition"
                    >
                      Next: Test Connection →
                    </button>
                  </div>
                </div>
              )}

              {wizardStep === 3 && (
                <div className="space-y-4">
                  <div className="p-3.5 rounded-xl bg-slate-900/80 border border-slate-800 font-mono text-xs space-y-2">
                    <div className="flex justify-between">
                      <span className="text-slate-500">Host:</span>
                      <span className="text-slate-200 font-bold">{formName} ({formAddress})</span>
                    </div>
                    <div className="flex justify-between">
                      <span className="text-slate-500">Endpoint:</span>
                      <span className="text-slate-300">{formAgentURL}</span>
                    </div>
                  </div>

                  {/* Connection Test Panel */}
                  <div className="p-4 rounded-xl border border-slate-800 bg-slate-900/40">
                    <div className="flex items-center justify-between mb-3">
                      <span className="text-xs font-mono font-bold text-slate-300 flex items-center gap-1.5">
                        <Activity className="w-4 h-4 text-cyan-400" />
                        Daemon Reachability Check
                      </span>
                      <button
                        type="button"
                        onClick={handleTestConnection}
                        disabled={testing}
                        className="px-2.5 py-1 rounded bg-slate-800 hover:bg-slate-700 text-[11px] font-mono text-slate-300 transition flex items-center gap-1"
                      >
                        <RefreshCw className={`w-3 h-3 ${testing ? 'animate-spin' : ''}`} />
                        Retest
                      </button>
                    </div>

                    {testing ? (
                      <div className="flex items-center gap-2 text-xs font-mono text-cyan-400 py-2">
                        <div className="w-3.5 h-3.5 border-2 border-cyan-400 border-t-transparent rounded-full animate-spin" />
                        <span>Verifying TLS, authentication and API version...</span>
                      </div>
                    ) : testResult ? (
                      testResult.success ? (
                        <div className="p-3 rounded-lg bg-emerald-950/40 border border-emerald-800/60 text-emerald-300 text-xs font-mono space-y-1">
                          <div className="flex items-center gap-1.5 font-bold">
                            <CheckCircle2 className="w-4 h-4 text-emerald-400" />
                            <span>✓ Connected Successfully ({testResult.latency_ms || 10}ms)</span>
                          </div>
                          <div>Daemon Version: {testResult.version || '1.5.0'}</div>
                          {testResult.region && <div>Region: {testResult.region}</div>}
                        </div>
                      ) : (
                        <div className="p-3 rounded-lg bg-rose-950/40 border border-rose-800/60 text-rose-300 text-xs font-mono space-y-1">
                          <div className="flex items-center gap-1.5 font-bold">
                            <XCircle className="w-4 h-4 text-rose-400" />
                            <span>✗ Connection Failed</span>
                          </div>
                          <div className="text-[11px] text-rose-400/90">{testResult.error}</div>
                        </div>
                      )
                    ) : null}
                  </div>

                  <label className="flex items-center gap-2 text-xs font-mono text-slate-300 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={formIsDefault}
                      onChange={(e) => setFormIsDefault(e.target.checked)}
                      className="rounded bg-slate-900 border-slate-700 text-cyan-500 focus:ring-cyan-500"
                    />
                    <span>Set this host as Default Host for the manager</span>
                  </label>

                  <div className="pt-2 flex justify-between">
                    <button
                      type="button"
                      onClick={() => setWizardStep(2)}
                      className="px-3 py-2 rounded-lg bg-slate-800 text-xs font-mono text-slate-300 hover:text-white"
                    >
                      ← Back
                    </button>
                    <button
                      type="submit"
                      disabled={saving}
                      className="px-5 py-2 rounded-lg bg-cyan-600 hover:bg-cyan-500 text-xs font-mono font-bold text-white transition disabled:opacity-50 shadow-lg shadow-cyan-950"
                    >
                      {saving ? 'Saving...' : 'Save & Register Host'}
                    </button>
                  </div>
                </div>
              )}
            </form>
          </div>
        </div>
      )}
    </div>
  );
};
