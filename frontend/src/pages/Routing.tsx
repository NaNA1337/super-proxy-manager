import React, { useState, useEffect } from 'react';
import {
  GitFork,
  ShieldCheck,
  Radio,
  RefreshCw,
  Terminal,
  Server,
  Layers,
  CheckCircle2,
  AlertOctagon
} from 'lucide-react';
import { RoutingOverview } from '../types';
import { StatusBadge } from '../components/StatusBadge';
import { api } from '../api/client';

export const Routing: React.FC = () => {
  const [routing, setRouting] = useState<RoutingOverview | null>(null);
  const [loading, setLoading] = useState(true);

  const fetchRouting = async () => {
    try {
      const res = await api.getRouting();
      setRouting(res);
    } catch (e) {
      console.error('Failed to load routing data', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchRouting();
    const interval = setInterval(fetchRouting, 5000);
    return () => clearInterval(interval);
  }, []);

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide">POLICY ROUTING & LEAK PREVENTION</h1>
          <p className="text-xs text-slate-400">
            Fwmark bindings, dedicated routing tables, and fail-closed firewall drop rules
          </p>
        </div>
        <button
          onClick={fetchRouting}
          className="flex items-center gap-1.5 px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 font-mono text-xs rounded-lg transition"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-cyan-400' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Security Leak Protection Status Bar */}
      <div className="grid grid-cols-1 sm:grid-cols-2 gap-4 font-mono text-xs">
        <div className="glass-panel p-4 rounded-xl border border-emerald-500/40 glow-emerald flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400">
              <ShieldCheck className="w-5 h-5" />
            </div>
            <div>
              <div className="font-bold text-white">DNS LEAK PROTECTION</div>
              <div className="text-[11px] text-slate-400">iptables DROP out of physical NIC on port 53</div>
            </div>
          </div>
          <span className="px-2.5 py-1 rounded bg-emerald-950 text-emerald-300 border border-emerald-500/50 font-bold">
            ENFORCED
          </span>
        </div>

        <div className="glass-panel p-4 rounded-xl border border-emerald-500/40 glow-emerald flex items-center justify-between">
          <div className="flex items-center gap-3">
            <div className="p-2 rounded-lg bg-emerald-500/10 text-emerald-400">
              <ShieldCheck className="w-5 h-5" />
            </div>
            <div>
              <div className="font-bold text-white">IPv6 LEAK PROTECTION</div>
              <div className="text-[11px] text-slate-400">ip6tables DROP all non-loopback forwards/output</div>
            </div>
          </div>
          <span className="px-2.5 py-1 rounded bg-emerald-950 text-emerald-300 border border-emerald-500/50 font-bold">
            FAIL-CLOSED
          </span>
        </div>
      </div>

      {/* Policy Routing Table Map */}
      <div className="glass-panel rounded-xl border border-slate-800 p-5 space-y-4">
        <div className="flex items-center justify-between">
          <div>
            <h3 className="text-sm font-bold text-white font-mono uppercase tracking-wider">
              POLICY ROUTING TABLE MAPPINGS
            </h3>
            <p className="text-xs text-slate-400">
              Correlates Xray outbound fwmark sockopts directly to per-slot routing tables and tun interfaces
            </p>
          </div>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-left font-mono text-xs">
            <thead className="bg-noc-900/80 text-slate-400 border-b border-slate-800 uppercase text-[11px]">
              <tr>
                <th className="py-3 px-4">Slot</th>
                <th className="py-3 px-4">Fwmark</th>
                <th className="py-3 px-4">Routing Table</th>
                <th className="py-3 px-4">Dev Interface</th>
                <th className="py-3 px-4">Assigned Node</th>
                <th className="py-3 px-4">Country</th>
                <th className="py-3 px-4">State</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800/60">
              {routing?.slots?.map((slot) => (
                <tr key={slot.slot} className="hover:bg-slate-800/30 transition">
                  <td className="py-3.5 px-4 font-bold text-cyan-400">Slot {slot.slot}</td>
                  <td className="py-3.5 px-4 text-slate-300 font-bold">{slot.fwmark}</td>
                  <td className="py-3.5 px-4 text-slate-300">table {slot.table_id}</td>
                  <td className="py-3.5 px-4 text-white font-bold">{slot.interface}</td>
                  <td className="py-3.5 px-4 text-slate-200">{slot.node_ip || 'Unbound'}</td>
                  <td className="py-3.5 px-4 text-cyan-300">{slot.country || '--'}</td>
                  <td className="py-3.5 px-4">
                    <StatusBadge status={slot.status} size="sm" />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>

      {/* Traffic Routing Explanation / Troubleshooting Card */}
      <div className="glass-panel p-5 rounded-xl border border-slate-800 font-mono text-xs space-y-3">
        <div className="flex items-center gap-2 text-slate-300 font-bold pb-2 border-b border-slate-800">
          <Terminal className="w-4 h-4 text-cyan-400" />
          <span>ROUTING LOGIC AUDIT: "WHY DOES TRAFFIC GO TO A SPECIFIC SLOT?"</span>
        </div>

        <div className="p-4 rounded-lg bg-noc-950 border border-slate-800 text-slate-300 space-y-2 leading-relaxed">
          <p>
            1. Client traffic enters Xray via <strong className="text-cyan-300">proxy (SOCKS/VLESS)</strong>.
          </p>
          <p>
            2. Xray's load balancer rules select an active outbound (<code className="text-amber-300">exit-0</code>, <code className="text-amber-300">exit-1</code>, or <code className="text-amber-300">exit-2</code>).
          </p>
          <p>
            3. Each outbound applies an SO_MARK: <code className="text-cyan-300">10000 + slot</code> to the socket.
          </p>
          <p>
            4. Linux kernel matches <code className="text-emerald-300">ip rule from all fwmark 1000X lookup 1000X</code> and forwards the packet into dedicated tun interface <code className="text-emerald-300">tunX</code>.
          </p>
          <p>
            5. During a manual or failover switch, the old tunnel is dynamically reassigned to draining table <code className="text-amber-300">2000X</code> while new connections immediately route to the new tunnel.
          </p>
        </div>
      </div>
    </div>
  );
};
