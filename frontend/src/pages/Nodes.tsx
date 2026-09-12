import React, { useState, useEffect } from 'react';
import {
  Search,
  Filter,
  ArrowUpDown,
  RefreshCw,
  Server,
  Globe,
  Zap,
  Activity,
  ChevronRight,
  ShieldAlert,
  ShieldCheck
} from 'lucide-react';
import { Node } from '../types';
import { StatusBadge } from '../components/StatusBadge';
import { NodeDetail } from './NodeDetail';
import { api } from '../api/client';

interface NodesProps {
  onSelectNodeForShareLink?: (node: Node) => void;
}

export const Nodes: React.FC<NodesProps> = ({ onSelectNodeForShareLink }) => {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [total, setTotal] = useState(0);
  const [loading, setLoading] = useState(true);
  const [search, setSearch] = useState('');
  const [countryFilter, setCountryFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState('');
  const [selectedNodeId, setSelectedNodeId] = useState<string | null>(null);

  // Sorting
  const [sortField, setSortField] = useState<'score' | 'speed' | 'rtt' | 'uptime'>('score');
  const [sortAsc, setSortAsc] = useState(false);

  const fetchNodes = async () => {
    setLoading(true);
    try {
      const res = await api.getNodes({
        search: search || undefined,
        country: countryFilter || undefined,
        status: statusFilter || undefined,
        limit: 150,
      });
      setNodes(res.nodes || []);
      setTotal(res.total || 0);
    } catch (e) {
      console.error('Failed to fetch nodes', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchNodes();
  }, [countryFilter, statusFilter]);

  const handleSearchSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    fetchNodes();
  };

  // Client-side sorting
  const sortedNodes = [...nodes].sort((a, b) => {
    let valA = 0;
    let valB = 0;
    if (sortField === 'score') {
      valA = a.score || 0;
      valB = b.score || 0;
    } else if (sortField === 'speed') {
      valA = a.performance?.download_bps || 0;
      valB = b.performance?.download_bps || 0;
    } else if (sortField === 'rtt') {
      valA = a.performance?.rtt_ms || 9999;
      valB = b.performance?.rtt_ms || 9999;
    } else if (sortField === 'uptime') {
      valA = a.uptime || 0;
      valB = b.uptime || 0;
    }
    return sortAsc ? valA - valB : valB - valA;
  });

  const toggleSort = (field: 'score' | 'speed' | 'rtt' | 'uptime') => {
    if (sortField === field) {
      setSortAsc(!sortAsc);
    } else {
      setSortField(field);
      setSortAsc(false);
    }
  };

  return (
    <div className="space-y-6">
      {/* Top Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide">NODE INVENTORY</h1>
          <p className="text-xs text-slate-400">
            Vetted exit candidates, reputation scores, and live operational metrics
          </p>
        </div>

        <div className="flex items-center gap-3">
          <span className="text-xs font-mono text-slate-400">
            Total Indexed: <strong className="text-cyan-400">{total}</strong>
          </span>
          <button
            onClick={fetchNodes}
            className="flex items-center gap-1.5 px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 font-mono text-xs rounded-lg transition"
          >
            <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-cyan-400' : ''}`} />
            <span>Refresh</span>
          </button>
        </div>
      </div>

      {/* Filter and Search Bar */}
      <div className="p-4 glass-panel rounded-xl border border-slate-800 flex flex-col md:flex-row gap-3 items-center justify-between">
        <form onSubmit={handleSearchSubmit} className="w-full md:w-80 relative">
          <Search className="w-4 h-4 text-slate-500 absolute left-3 top-2.5" />
          <input
            type="text"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="Search IP, Hostname, ASN, ISP..."
            className="w-full pl-9 pr-3 py-2 bg-noc-900 border border-slate-700 rounded-lg text-xs text-white placeholder-slate-500 focus:outline-none focus:border-cyan-500 font-mono"
          />
        </form>

        <div className="flex flex-wrap gap-2 w-full md:w-auto items-center">
          {/* Country Filter */}
          <select
            value={countryFilter}
            onChange={(e) => setCountryFilter(e.target.value)}
            className="px-3 py-2 bg-noc-900 border border-slate-700 rounded-lg text-xs text-slate-200 font-mono focus:outline-none focus:border-cyan-500"
          >
            <option value="">All Countries</option>
            <option value="JP">Japan (JP)</option>
            <option value="US">United States (US)</option>
            <option value="KR">South Korea (KR)</option>
            <option value="SG">Singapore (SG)</option>
            <option value="DE">Germany (DE)</option>
            <option value="GB">United Kingdom (GB)</option>
          </select>

          {/* Status Filter */}
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            className="px-3 py-2 bg-noc-900 border border-slate-700 rounded-lg text-xs text-slate-200 font-mono focus:outline-none focus:border-cyan-500"
          >
            <option value="">All Statuses</option>
            <option value="ACTIVE">ACTIVE</option>
            <option value="STANDBY">STANDBY</option>
            <option value="QUALIFIED">QUALIFIED</option>
            <option value="DISCOVERED">DISCOVERED</option>
            <option value="COOLDOWN">COOLDOWN</option>
            <option value="FAILED">FAILED</option>
          </select>
        </div>
      </div>

      {/* Nodes Table */}
      <div className="glass-panel rounded-xl border border-slate-800 overflow-hidden">
        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-xs">
            <thead className="bg-noc-900/80 text-slate-400 border-b border-slate-800 uppercase text-[11px] tracking-wider">
              <tr>
                <th className="py-3.5 px-4 font-semibold">Node Endpoint</th>
                <th className="py-3.5 px-4 font-semibold">Country</th>
                <th className="py-3.5 px-4 font-semibold">
                  <button
                    onClick={() => toggleSort('score')}
                    className="flex items-center gap-1 hover:text-white"
                  >
                    <span>FSM Score</span>
                    <ArrowUpDown className="w-3 h-3 text-cyan-400" />
                  </button>
                </th>
                <th className="py-3.5 px-4 font-semibold">Status</th>
                <th className="py-3.5 px-4 font-semibold">
                  <button
                    onClick={() => toggleSort('speed')}
                    className="flex items-center gap-1 hover:text-white"
                  >
                    <span>Throughput</span>
                    <ArrowUpDown className="w-3 h-3 text-emerald-400" />
                  </button>
                </th>
                <th className="py-3.5 px-4 font-semibold">
                  <button
                    onClick={() => toggleSort('rtt')}
                    className="flex items-center gap-1 hover:text-white"
                  >
                    <span>RTT</span>
                    <ArrowUpDown className="w-3 h-3 text-amber-400" />
                  </button>
                </th>
                <th className="py-3.5 px-4 font-semibold">ASN / Provider</th>
                <th className="py-3.5 px-4 font-semibold">Reputation</th>
                <th className="py-3.5 px-4 font-semibold text-right">Details</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {loading && sortedNodes.length === 0 ? (
                <tr>
                  <td colSpan={9} className="text-center py-12 text-slate-500">
                    Querying node inventory...
                  </td>
                </tr>
              ) : sortedNodes.length === 0 ? (
                <tr>
                  <td colSpan={9} className="text-center py-12 text-slate-500">
                    No nodes match current filter parameters.
                  </td>
                </tr>
              ) : (
                sortedNodes.map((node) => (
                  <tr
                    key={node.id}
                    onClick={() => setSelectedNodeId(node.id)}
                    className="hover:bg-slate-800/40 cursor-pointer transition"
                  >
                    <td className="py-3 px-4 font-bold text-white">
                      <div>{node.ip}</div>
                      <div className="text-[10px] text-slate-500 truncate max-w-xs font-normal">
                        {node.hostname || node.id}
                      </div>
                    </td>

                    <td className="py-3 px-4 text-cyan-300 font-semibold">
                      {node.country || 'N/A'}
                    </td>

                    <td className="py-3 px-4">
                      <span className="font-bold text-amber-400">{node.score}</span>
                    </td>

                    <td className="py-3 px-4">
                      <StatusBadge status={node.status} size="sm" />
                    </td>

                    <td className="py-3 px-4 text-emerald-400 font-bold">
                      {node.performance?.download_bps > 0
                        ? `${(node.performance.download_bps / 1_000_000).toFixed(1)} Mbps`
                        : '--'}
                    </td>

                    <td className="py-3 px-4 text-slate-300">
                      {node.performance?.rtt_ms > 0 ? `${node.performance.rtt_ms} ms` : '--'}
                    </td>

                    <td className="py-3 px-4 text-slate-300 truncate max-w-xs">
					  {node.network_class?.asn || 'ASN pending'} ({node.network_class?.isp || 'Provider unavailable'})
                    </td>

                    <td className="py-3 px-4">
                      {node.reputation?.status === 'GOOD' ? (
                        <span className="inline-flex items-center gap-1 text-emerald-400 text-xs font-semibold">
                          <ShieldCheck className="w-3.5 h-3.5" /> GOOD
                        </span>
                      ) : (
                        <span className="inline-flex items-center gap-1 text-amber-400 text-xs">
                          <ShieldAlert className="w-3.5 h-3.5" /> {node.reputation?.status || 'UNKNOWN'}
                        </span>
                      )}
                    </td>

                    <td className="py-3 px-4 text-right">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          setSelectedNodeId(node.id);
                        }}
                        className="p-1.5 text-slate-400 hover:text-cyan-400 hover:bg-slate-800 rounded transition"
                      >
                        <ChevronRight className="w-4 h-4" />
                      </button>
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      </div>

      {/* Node Detail Drawer / Modal */}
      {selectedNodeId && (
        <NodeDetail
          nodeId={selectedNodeId}
          onClose={() => setSelectedNodeId(null)}
          onSelectForSwitch={(node) => {
            setSelectedNodeId(null);
            if (onSelectNodeForShareLink) {
              onSelectNodeForShareLink(node);
            }
          }}
        />
      )}
    </div>
  );
};
