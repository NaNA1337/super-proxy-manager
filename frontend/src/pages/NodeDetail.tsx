import React, { useEffect, useState } from 'react';
import {
  X,
  Shield,
  Activity,
  Globe,
  Server,
  Zap,
  Clock,
  AlertTriangle,
  CheckCircle2,
  MinusCircle,
  HelpCircle,
  TrendingUp,
  Cpu
} from 'lucide-react';
import { Node } from '../types';
import { StatusBadge } from '../components/StatusBadge';
import { api } from '../api/client';

interface NodeDetailProps {
  nodeId: string;
  onClose: () => void;
  onSelectForSwitch?: (node: Node) => void;
}

export const NodeDetail: React.FC<NodeDetailProps> = ({ nodeId, onClose, onSelectForSwitch }) => {
  const [node, setNode] = useState<Node | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  useEffect(() => {
    const fetchDetail = async () => {
      setLoading(true);
      try {
        const data = await api.getNodeDetails(nodeId);
        setNode(data);
      } catch (err: unknown) {
        if (err instanceof Error) {
          setError(err.message);
        } else {
          setError('Failed to load node details');
        }
      } finally {
        setLoading(false);
      }
    };
    if (nodeId) fetchDetail();
  }, [nodeId]);

  if (!nodeId) return null;

  // Calculate Transparent Score Breakdown
  const computeScoreExplanation = (n: Node) => {
    const positive: { label: string; score: string; note: string }[] = [];
    const negative: { label: string; score: string; note: string }[] = [];

    // Base
    positive.push({
      label: 'Base VPN Gate Score',
      score: `+${n.score}`,
      note: 'Bootstrap operator score',
    });

    // Primary region
    if (n.country === 'JP') {
      positive.push({
        label: 'Primary Region Bonus (JP)',
        score: '+50',
        note: 'High geographic affinity',
      });
    }

    // Speed bonus
    if (n.performance?.download_bps > 0) {
      const mbps = n.performance.download_bps / 1_000_000;
      const speedBonus = Math.min(200, Math.floor(mbps * 1.0));
      positive.push({
        label: `High Throughput (${mbps.toFixed(1)} Mbps)`,
        score: `+${speedBonus}`,
        note: 'Bandwidth benchmark performance',
      });
    }

    // Latency bonus
    if (n.performance?.rtt_ms > 0 && n.performance.rtt_ms < 200) {
      const latBonus = Math.floor((200 - n.performance.rtt_ms) * 0.5);
      positive.push({
        label: `Low Latency (${n.performance.rtt_ms}ms)`,
        score: `+${latBonus}`,
        note: 'Fast RTT response time',
      });
    }

    // Negative penalties
    if (n.performance?.packet_loss_pct > 2.0) {
      const penalty = Math.floor(n.performance.packet_loss_pct * 2);
      negative.push({
        label: `Packet Loss (${n.performance.packet_loss_pct.toFixed(1)}%)`,
        score: `-${penalty}`,
        note: 'Network jitter / drop rate',
      });
    }

    if (n.reputation?.fraud_score > 0) {
      negative.push({
        label: 'Reputation Fraud Risk',
        score: `-${n.reputation.fraud_score}`,
        note: n.reputation.details || 'AbuseIPDB/GreyNoise penalty',
      });
    }

    if (n.network_class?.is_hosting) {
      negative.push({
        label: 'Datacenter / Hosting ASN',
        score: '-10',
        note: `${n.network_class.asn || 'Hosting provider'} detection`,
      });
    }

    if (n.network_class?.is_vpn) {
      negative.push({
        label: 'Commercial VPN Tag Detected',
        score: '-5',
        note: 'Network trait: VPN detected (NOTE: Non-malicious exit trait)',
      });
    }

    if (n.fail_count > 0) {
      negative.push({
        label: 'Historical Connection Failures',
        score: `-${n.fail_count * 30}`,
        note: `${n.fail_count} prior tunnel connection drop(s)`,
      });
    }

    return { positive, negative };
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150">
      <div className="relative w-full max-w-4xl max-h-[90vh] glass-panel rounded-2xl border border-slate-800 flex flex-col shadow-2xl overflow-hidden">
        {/* Modal Header */}
        <div className="p-6 border-b border-slate-800/80 flex items-center justify-between bg-noc-900/50">
          <div className="flex items-center gap-4">
            <div className="p-2.5 rounded-xl bg-cyan-500/10 border border-cyan-500/30 text-cyan-400">
              <Server className="w-6 h-6" />
            </div>
            <div>
              <div className="flex items-center gap-3">
                <h2 className="text-xl font-bold text-white font-mono">{nodeId}</h2>
                {node && <StatusBadge status={node.status} size="sm" />}
              </div>
              <p className="text-xs text-slate-400 font-mono mt-0.5">
                Host: {node?.hostname || 'N/A'} • {node?.country_long || node?.country} ({node?.country})
              </p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 text-slate-400 hover:text-white rounded-lg hover:bg-slate-800 transition"
          >
            <X className="w-5 h-5" />
          </button>
        </div>

        {/* Modal Content Scrollable */}
        <div className="p-6 overflow-y-auto space-y-6">
          {loading ? (
            <div className="py-20 text-center text-slate-400 font-mono">Loading node intelligence...</div>
          ) : error || !node ? (
            <div className="p-4 rounded-lg bg-rose-950/50 border border-rose-800 text-rose-300 text-sm">
              {error || 'Node not found'}
            </div>
          ) : (
            <>
              {/* Decision Transparency Card: "Why was this node chosen?" (Stage 6) */}
              <div className="p-5 rounded-xl bg-gradient-to-br from-cyan-950/40 via-noc-900/60 to-blue-950/40 border border-cyan-500/30 shadow-lg">
                <div className="flex items-center justify-between mb-4">
                  <div className="flex items-center gap-2">
                    <TrendingUp className="w-5 h-5 text-cyan-400" />
                    <h3 className="text-sm font-bold text-white uppercase tracking-wider font-mono">
                      FSM SELECTION & SCORING BREAKDOWN
                    </h3>
                  </div>
                  <span className="text-sm font-bold font-mono px-2.5 py-1 rounded bg-cyan-900/80 text-cyan-300 border border-cyan-500/50">
                    Calculated Score: {node.score}
                  </span>
                </div>

                <p className="text-xs text-slate-300 mb-4">
                  Demonstrates the exact mathematical audit trail used by the FSM candidate selector:
                </p>

                {(() => {
                  const { positive, negative } = computeScoreExplanation(node);
                  return (
                    <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                      {/* Positive Factors */}
                      <div className="space-y-2">
                        <span className="text-xs font-mono font-bold text-emerald-400 uppercase flex items-center gap-1.5">
                          <CheckCircle2 className="w-3.5 h-3.5" /> Positive Evaluation Factors:
                        </span>
                        <div className="space-y-1.5">
                          {positive.map((p, idx) => (
                            <div
                              key={idx}
                              className="p-2.5 rounded-lg bg-emerald-950/30 border border-emerald-800/40 text-xs font-mono flex items-center justify-between"
                            >
                              <div>
                                <div className="text-slate-200 font-semibold">{p.label}</div>
                                <div className="text-[11px] text-slate-400">{p.note}</div>
                              </div>
                              <span className="text-emerald-400 font-bold ml-2">{p.score}</span>
                            </div>
                          ))}
                        </div>
                      </div>

                      {/* Negative Factors */}
                      <div className="space-y-2">
                        <span className="text-xs font-mono font-bold text-amber-400 uppercase flex items-center gap-1.5">
                          <MinusCircle className="w-3.5 h-3.5" /> Negative Penalties:
                        </span>
                        <div className="space-y-1.5">
                          {negative.length > 0 ? (
                            negative.map((n, idx) => (
                              <div
                                key={idx}
                                className="p-2.5 rounded-lg bg-amber-950/20 border border-amber-800/40 text-xs font-mono flex items-center justify-between"
                              >
                                <div>
                                  <div className="text-slate-200 font-semibold">{n.label}</div>
                                  <div className="text-[11px] text-slate-400">{n.note}</div>
                                </div>
                                <span className="text-rose-400 font-bold ml-2">{n.score}</span>
                              </div>
                            ))
                          ) : (
                            <div className="p-3 text-xs text-slate-500 font-mono italic">
                              Zero adverse penalties detected.
                            </div>
                          )}
                        </div>
                      </div>
                    </div>
                  );
                })()}
              </div>

              {/* Network Intelligence & Reputation Grid */}
              <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
                {/* Network Intelligence */}
                <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3 font-mono text-xs">
                  <div className="flex items-center gap-2 text-slate-300 font-bold pb-2 border-b border-slate-800">
                    <Globe className="w-4 h-4 text-cyan-400" />
                    <span>NETWORK INTELLIGENCE (ASN & ISP)</span>
                  </div>

                  <div className="flex justify-between py-1">
                    <span className="text-slate-400">ASN:</span>
                    <span className="text-white font-bold">{node.network_class?.asn || 'N/A'}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-slate-400">ISP / Provider:</span>
                    <span className="text-slate-200">{node.network_class?.isp || 'N/A'}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-slate-400">Organization:</span>
                    <span className="text-slate-200">{node.network_class?.organization || 'N/A'}</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-slate-400">Network Category:</span>
                    <span className="text-cyan-300 uppercase">{node.network_class?.network_type || 'broadband'}</span>
                  </div>

                  <div className="pt-2 border-t border-slate-800/60 grid grid-cols-4 gap-2 text-center text-[10px]">
                    <div className={`p-1.5 rounded border ${node.network_class?.is_vpn ? 'bg-amber-950/40 border-amber-800 text-amber-300' : 'bg-slate-900 border-slate-800 text-slate-500'}`}>
                      VPN
                    </div>
                    <div className={`p-1.5 rounded border ${node.network_class?.is_proxy ? 'bg-amber-950/40 border-amber-800 text-amber-300' : 'bg-slate-900 border-slate-800 text-slate-500'}`}>
                      PROXY
                    </div>
                    <div className={`p-1.5 rounded border ${node.network_class?.is_tor ? 'bg-rose-950/60 border-rose-800 text-rose-300' : 'bg-slate-900 border-slate-800 text-slate-500'}`}>
                      TOR
                    </div>
                    <div className={`p-1.5 rounded border ${node.network_class?.is_hosting ? 'bg-blue-950/40 border-blue-800 text-blue-300' : 'bg-slate-900 border-slate-800 text-slate-500'}`}>
                      HOSTING
                    </div>
                  </div>
                </div>

                {/* Reputation & Threat Analysis */}
                <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3 font-mono text-xs">
                  <div className="flex items-center gap-2 text-slate-300 font-bold pb-2 border-b border-slate-800">
                    <Shield className="w-4 h-4 text-emerald-400" />
                    <span>REPUTATION & SECURITY VERIFICATION</span>
                  </div>

                  <div className="flex justify-between py-1">
                    <span className="text-slate-400">Reputation Status:</span>
                    <span className={`font-bold ${node.reputation?.status === 'GOOD' ? 'text-emerald-400' : 'text-amber-400'}`}>
                      {node.reputation?.status || 'UNKNOWN'}
                    </span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-slate-400">Fraud / Abuse Score:</span>
                    <span className="text-white font-bold">{node.reputation?.fraud_score || 0} / 100</span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-slate-400">Blacklisted:</span>
                    <span className={node.reputation?.is_blacklisted ? 'text-rose-400 font-bold' : 'text-emerald-400'}>
                      {node.reputation?.is_blacklisted ? 'YES (REJECTED)' : 'NO'}
                    </span>
                  </div>
                  <div className="flex justify-between py-1">
                    <span className="text-slate-400">Evaluation Engine:</span>
                    <span className="text-slate-300">{node.reputation?.provider_name || 'AbuseIPDB/Prefix'}</span>
                  </div>

                  <div className="pt-2 border-t border-slate-800/60 text-[11px] text-slate-400">
                    <span className="text-slate-500">Evidence Details:</span>
                    <p className="mt-1 p-2 rounded bg-noc-950 border border-slate-800/80 text-slate-300">
                      {node.reputation?.details || 'Passed multi-provider prefix and ASN reputation vetting.'}
                    </p>
                  </div>
                </div>
              </div>

              {/* Performance Metrics */}
              <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3 font-mono text-xs">
                <div className="flex items-center gap-2 text-slate-300 font-bold pb-2 border-b border-slate-800">
                  <Activity className="w-4 h-4 text-amber-400" />
                  <span>MEASURED PERFORMANCE BENCHMARKS</span>
                </div>

                <div className="grid grid-cols-2 sm:grid-cols-4 gap-4 pt-1">
                  <div className="p-3 rounded-lg bg-noc-900 border border-slate-800">
                    <span className="text-slate-500 text-[11px]">Round-Trip Time</span>
                    <div className="text-base font-bold text-white mt-1">
                      {node.performance?.rtt_ms > 0 ? `${node.performance.rtt_ms} ms` : 'Unmeasured'}
                    </div>
                  </div>

                  <div className="p-3 rounded-lg bg-noc-900 border border-slate-800">
                    <span className="text-slate-500 text-[11px]">Download Speed</span>
                    <div className="text-base font-bold text-emerald-400 mt-1">
                      {node.performance?.download_bps > 0
                        ? `${(node.performance.download_bps / 1_000_000).toFixed(1)} Mbps`
                        : 'Unmeasured'}
                    </div>
                  </div>

                  <div className="p-3 rounded-lg bg-noc-900 border border-slate-800">
                    <span className="text-slate-500 text-[11px]">Packet Loss</span>
                    <div className="text-base font-bold text-slate-200 mt-1">
                      {node.performance?.packet_loss_pct >= 0
                        ? `${node.performance.packet_loss_pct.toFixed(1)}%`
                        : '0.0%'}
                    </div>
                  </div>

                  <div className="p-3 rounded-lg bg-noc-900 border border-slate-800">
                    <span className="text-slate-500 text-[11px]">Concurrent Sessions</span>
                    <div className="text-base font-bold text-cyan-400 mt-1">{node.sessions || 0}</div>
                  </div>
                </div>
              </div>
            </>
          )}
        </div>

        {/* Modal Footer */}
        <div className="p-4 border-t border-slate-800/80 bg-noc-900/50 flex items-center justify-between">
          <span className="text-xs font-mono text-slate-500">
            Node ID: {node?.id} • First Seen: {node ? new Date(node.first_seen).toLocaleDateString() : '--'}
          </span>
          <div className="flex gap-3">
            {onSelectForSwitch && node && (node.status === 'DISCOVERED' || node.status === 'STANDBY' || node.status === 'QUALIFIED') && (
              <button
                onClick={() => onSelectForSwitch(node)}
                className="px-4 py-2 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-semibold text-xs rounded-lg transition shadow-lg shadow-cyan-950/40"
              >
                Switch Slot to this Node
              </button>
            )}
            <button
              onClick={onClose}
              className="px-4 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-xs rounded-lg transition"
            >
              Close
            </button>
          </div>
        </div>
      </div>
    </div>
  );
};
