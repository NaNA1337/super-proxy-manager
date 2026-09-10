import React, { useState, useEffect } from 'react';
import {
  Rss,
  Plus,
  Copy,
  Check,
  Trash2,
  AlertCircle,
  Clock,
  ShieldCheck,
  RefreshCw,
  Key,
  Globe
} from 'lucide-react';
import { Subscription, CreateSubResponse } from '../types';
import { api } from '../api/client';

interface SubscriptionsProps {
  isAdmin: boolean;
}

export const Subscriptions: React.FC<SubscriptionsProps> = ({ isAdmin }) => {
  const [subscriptions, setSubscriptions] = useState<Subscription[]>([]);
  const [loading, setLoading] = useState(true);
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // New Subscription Form
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newSubName, setNewSubName] = useState('');
  const [newProfile, setNewProfile] = useState<'active_only' | 'all_nodes' | 'region' | 'protocol'>('active_only');
  const [newRegion, setNewRegion] = useState('JP');
  const [newProtocol, setNewProtocol] = useState('socks5');
  const [newDuration, setNewDuration] = useState(30);
  const [createLoading, setCreateLoading] = useState(false);
  const [createError, setCreateError] = useState('');
  const [createdResponse, setCreatedResponse] = useState<CreateSubResponse | null>(null);

  const fetchSubscriptions = async () => {
    try {
      const data = await api.listSubscriptions();
      setSubscriptions(data || []);
    } catch (e) {
      console.error('Failed to load subscriptions', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSubscriptions();
  }, []);

  const handleCreate = async (e: React.FormEvent) => {
    e.preventDefault();
    setCreateLoading(true);
    setCreateError('');

    try {
      const res = await api.createSubscription({
        name: newSubName,
        profile: newProfile,
        region: newProfile === 'region' ? newRegion : undefined,
        protocol: newProfile === 'protocol' ? newProtocol : undefined,
        duration_days: newDuration > 0 ? newDuration : undefined,
      });
      setCreatedResponse(res);
      fetchSubscriptions();
    } catch (err: unknown) {
      if (err instanceof Error) {
        setCreateError(err.message);
      } else {
        setCreateError('Failed to create subscription');
      }
    } finally {
      setCreateLoading(false);
    }
  };

  const handleRevoke = async (id: string) => {
    if (!confirm('Are you sure you want to revoke this subscription? Any client using this token will be immediately blocked.')) {
      return;
    }
    try {
      await api.revokeSubscription(id);
      fetchSubscriptions();
    } catch (e) {
      console.error('Failed to revoke subscription', e);
    }
  };

  const handleCopyLink = async (text: string, id: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 2000);
    } catch (e) {
      console.error('Failed to copy', e);
    }
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide">CLIENT SUBSCRIPTIONS</h1>
          <p className="text-xs text-slate-400">
            Automated base64 proxy list endpoints compatible with Clash, V2Ray, Shadowrocket, and Sing-box
          </p>
        </div>

        {isAdmin && (
          <button
            onClick={() => {
              setShowCreateModal(true);
              setCreatedResponse(null);
              setNewSubName('');
            }}
            className="flex items-center gap-2 px-4 py-2.5 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-semibold text-xs rounded-lg shadow-lg shadow-cyan-950/40 transition"
          >
            <Plus className="w-4 h-4" />
            <span>Generate New Subscription</span>
          </button>
        )}
      </div>

      {/* Subscriptions Table */}
      <div className="glass-panel rounded-xl border border-slate-800 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-xs">
            <thead className="bg-noc-900/80 text-slate-400 border-b border-slate-800 uppercase text-[11px]">
              <tr>
                <th className="py-3 px-4">Subscription Name</th>
                <th className="py-3 px-4">Profile Type</th>
                <th className="py-3 px-4">Filter Rules</th>
                <th className="py-3 px-4">Status</th>
                <th className="py-3 px-4">Created By</th>
                <th className="py-3 px-4">Expires</th>
                <th className="py-3 px-4 text-right">Actions</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {subscriptions.length === 0 ? (
                <tr>
                  <td colSpan={7} className="text-center py-12 text-slate-500">
                    No subscriptions generated yet.
                  </td>
                </tr>
              ) : (
                subscriptions.map((sub) => (
                  <tr key={sub.id} className="hover:bg-slate-800/30 transition">
                    <td className="py-3.5 px-4 font-bold text-white">
                      <div>{sub.name || 'Unnamed Subscription'}</div>
                      <div className="text-[10px] text-slate-500 font-normal">ID: {sub.id}</div>
                    </td>

                    <td className="py-3.5 px-4 text-cyan-300 uppercase">
                      {sub.profile.replace('_', ' ')}
                    </td>

                    <td className="py-3.5 px-4 text-slate-300">
                      {sub.profile === 'region' && sub.region_filter ? (
                        <span className="px-2 py-0.5 rounded bg-blue-950 text-blue-300 border border-blue-800">
                          Region: {sub.region_filter}
                        </span>
                      ) : sub.profile === 'protocol' && sub.protocol_filter ? (
                        <span className="px-2 py-0.5 rounded bg-purple-950 text-purple-300 border border-purple-800">
                          Proto: {sub.protocol_filter}
                        </span>
                      ) : (
                        <span className="text-slate-500">All Nodes</span>
                      )}
                    </td>

                    <td className="py-3.5 px-4">
                      {sub.is_revoked ? (
                        <span className="px-2 py-0.5 rounded bg-rose-950 text-rose-400 border border-rose-800 font-bold uppercase text-[10px]">
                          REVOKED (403)
                        </span>
                      ) : (
                        <span className="px-2 py-0.5 rounded bg-emerald-950 text-emerald-300 border border-emerald-500/50 font-bold uppercase text-[10px]">
                          ACTIVE
                        </span>
                      )}
                    </td>

                    <td className="py-3.5 px-4 text-slate-400">{sub.created_by}</td>

                    <td className="py-3.5 px-4 text-slate-400">
                      {sub.expires_at ? new Date(sub.expires_at).toLocaleDateString() : 'Never'}
                    </td>

                    <td className="py-3.5 px-4 text-right">
                      {isAdmin && !sub.is_revoked && (
                        <button
                          onClick={() => handleRevoke(sub.id)}
                          className="p-1.5 text-rose-400 hover:text-rose-300 hover:bg-rose-950/40 rounded transition"
                          title="Revoke Token"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                      )}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Create Subscription Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150">
          <div className="w-full max-w-lg glass-panel p-6 rounded-2xl border border-slate-700 shadow-2xl space-y-4">
            <h3 className="text-base font-bold text-white font-mono uppercase tracking-wider pb-3 border-b border-slate-800">
              CREATE CLIENT SUBSCRIPTION
            </h3>

            {createError && (
              <div className="p-3 rounded-lg bg-rose-950/60 border border-rose-800 text-rose-300 text-xs flex items-center gap-2 font-mono">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{createError}</span>
              </div>
            )}

            {createdResponse ? (
              <div className="space-y-4 font-mono text-xs">
                <div className="p-3 rounded-lg bg-emerald-950/60 border border-emerald-500/50 text-emerald-200">
                  <div className="font-bold flex items-center gap-1.5">
                    <Check className="w-4 h-4 text-emerald-400" />
                    <span>SUBSCRIPTION GENERATED SUCCESSFULLY!</span>
                  </div>
                  <p className="text-[11px] text-slate-300 mt-1">
                    Copy the subscription URL below. The secret token is displayed only once and stored strictly as a SHA-256 hash in the database.
                  </p>
                </div>

                <div>
                  <label className="block text-slate-400 mb-1">Subscription URL:</label>
                  <div className="p-3 rounded-lg bg-noc-950 border border-slate-800 text-cyan-300 break-all select-all font-semibold">
                    {createdResponse.sub_url}
                  </div>
                </div>

                <div className="flex gap-3 pt-2">
                  <button
                    onClick={() => handleCopyLink(createdResponse.sub_url, 'new')}
                    className="flex-1 py-2.5 px-4 bg-cyan-600 hover:bg-cyan-500 text-white font-semibold rounded-lg transition flex items-center justify-center gap-1.5"
                  >
                    {copiedId === 'new' ? <Check className="w-4 h-4" /> : <Copy className="w-4 h-4" />}
                    <span>{copiedId === 'new' ? 'Copied!' : 'Copy Subscription Link'}</span>
                  </button>
                  <button
                    onClick={() => setShowCreateModal(false)}
                    className="py-2.5 px-4 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium rounded-lg transition"
                  >
                    Done
                  </button>
                </div>
              </div>
            ) : (
              <form onSubmit={handleCreate} className="space-y-4 font-mono text-xs">
                <div>
                  <label className="block text-slate-400 mb-1">Subscription Name / Description:</label>
                  <input
                    type="text"
                    required
                    value={newSubName}
                    onChange={(e) => setNewSubName(e.target.value)}
                    placeholder="e.g. Tokyo Workstation Clash Sub"
                    className="w-full px-3 py-2 bg-noc-900 border border-slate-700 rounded-lg text-white placeholder-slate-500 focus:outline-none focus:border-cyan-500"
                  />
                </div>

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-slate-400 mb-1">Profile Scope:</label>
                    <select
                      value={newProfile}
                      onChange={(e) => setNewProfile(e.target.value as any)}
                      className="w-full px-3 py-2 bg-noc-900 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                    >
                      <option value="active_only">Active Slots Only (Warm)</option>
                      <option value="all_nodes">All Qualified Nodes</option>
                      <option value="region">Region Filtered</option>
                      <option value="protocol">Protocol Filtered</option>
                    </select>
                  </div>

                  <div>
                    <label className="block text-slate-400 mb-1">Valid Duration (Days):</label>
                    <input
                      type="number"
                      value={newDuration}
                      onChange={(e) => setNewDuration(parseInt(e.target.value) || 0)}
                      className="w-full px-3 py-2 bg-noc-900 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                </div>

                {newProfile === 'region' && (
                  <div>
                    <label className="block text-slate-400 mb-1">Target Region Country Code:</label>
                    <input
                      type="text"
                      value={newRegion}
                      onChange={(e) => setNewRegion(e.target.value.toUpperCase())}
                      placeholder="JP, US, KR, etc."
                      className="w-full px-3 py-2 bg-noc-900 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                    />
                  </div>
                )}

                {newProfile === 'protocol' && (
                  <div>
                    <label className="block text-slate-400 mb-1">Target Protocol:</label>
                    <select
                      value={newProtocol}
                      onChange={(e) => setNewProtocol(e.target.value)}
                      className="w-full px-3 py-2 bg-noc-900 border border-slate-700 rounded-lg text-white focus:outline-none focus:border-cyan-500"
                    >
                      <option value="vless">VLESS Reality</option>
                      <option value="socks5">SOCKS5</option>
                    </select>
                  </div>
                )}

                <div className="flex gap-3 pt-3 border-t border-slate-800">
                  <button
                    type="submit"
                    disabled={createLoading}
                    className="flex-1 py-2.5 px-4 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-semibold rounded-lg transition disabled:opacity-40"
                  >
                    {createLoading ? 'Generating Token...' : 'Generate Subscription'}
                  </button>
                  <button
                    type="button"
                    onClick={() => setShowCreateModal(false)}
                    className="py-2.5 px-4 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium rounded-lg transition"
                  >
                    Cancel
                  </button>
                </div>
              </form>
            )}
          </div>
        </div>
      )}
    </div>
  );
};
