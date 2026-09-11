import React, { useState, useEffect } from 'react';
import {
  Layers,
  ArrowRightLeft,
  Shield,
  Activity,
  AlertCircle,
  CheckCircle2,
  RefreshCw,
  Lock,
  Clock
} from 'lucide-react';
import { SlotsOverview, CurrentExit, Node, Operation } from '../types';
import { StatusBadge } from '../components/StatusBadge';
import { api } from '../api/client';

interface SlotsProps {
  isAdmin: boolean;
}

export const Slots: React.FC<SlotsProps> = ({ isAdmin }) => {
  const [slotsOverview, setSlotsOverview] = useState<SlotsOverview | null>(null);
  const [exits, setExits] = useState<CurrentExit[]>([]);
  const [qualifiedNodes, setQualifiedNodes] = useState<Node[]>([]);
  const [loading, setLoading] = useState(true);

  // Switch Modal State
  const [switchingSlot, setSwitchingSlot] = useState<number | null>(null);
  const [selectedTargetNode, setSelectedTargetNode] = useState<string>('');
  const [switchLoading, setSwitchLoading] = useState(false);
  const [switchError, setSwitchError] = useState('');
  const [activeOp, setActiveOp] = useState<Operation | null>(null);

  const fetchSlotsData = async () => {
    try {
      const [slotsRes, exitsRes, qualifiedRes] = await Promise.all([
        api.getSlots(),
        api.getCurrentExits().catch(() => []),
        api.getPoolQualified().catch(() => []),
      ]);
      setSlotsOverview(slotsRes);
      setExits(exitsRes);
      setQualifiedNodes(qualifiedRes);
    } catch (e) {
      console.error('Failed to fetch slots data', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchSlotsData();
    const interval = setInterval(fetchSlotsData, 3000);
    return () => clearInterval(interval);
  }, []);

  // Poll active operation status
  useEffect(() => {
    if (!activeOp || activeOp.status === 'ACTIVE' || activeOp.status === 'FAILED') {
      return;
    }
    const timer = setInterval(async () => {
      try {
        const updated = await api.getOperation(activeOp.operation_id);
        setActiveOp(updated);
        if (updated.status === 'ACTIVE' || updated.status === 'FAILED') {
          fetchSlotsData();
        }
      } catch (e) {
        console.error('Failed to poll operation', e);
      }
    }, 1500);
    return () => clearInterval(timer);
  }, [activeOp]);

  const handleStartSwitch = (slot: number) => {
    setSwitchingSlot(slot);
    setSelectedTargetNode('');
    setSwitchError('');
  };

  const handleExecuteSwitch = async () => {
    if (switchingSlot === null || !selectedTargetNode) return;
    setSwitchLoading(true);
    setSwitchError('');

    try {
      const res = await api.switchSlot(switchingSlot, selectedTargetNode);
      if (res.operation_id) {
        setActiveOp({
          operation_id: res.operation_id,
          slot: switchingSlot,
          target_node_id: selectedTargetNode,
          status: 'REQUESTED',
          created_at: new Date().toISOString(),
          updated_at: new Date().toISOString(),
        });
        setSwitchingSlot(null);
      }
    } catch (err: unknown) {
      if (err instanceof Error) {
        setSwitchError(err.message);
      } else {
        setSwitchError('Failed to initiate switch');
      }
    } finally {
      setSwitchLoading(false);
    }
  };

  const totalSlots = slotsOverview?.total_configured || 3;
  const slotList = [];
  for (let i = 0; i < totalSlots; i++) {
    const exit = exits.find((e) => e.slot === i);
    slotList.push({
      slot: i,
      exit: exit || null,
      isLocked: slotsOverview?.manual_overrides?.[i] || false,
    });
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide">SLOT CONTROLLER</h1>
          <p className="text-xs text-slate-400">
            Dedicated Linux policy routing tunnels, atomic generation leases, and graceful draining state machines
          </p>
        </div>
        <button
          onClick={fetchSlotsData}
          className="flex items-center gap-1.5 px-3 py-2 bg-slate-800 hover:bg-slate-700 text-slate-300 font-mono text-xs rounded-lg transition"
        >
          <RefreshCw className={`w-3.5 h-3.5 ${loading ? 'animate-spin text-cyan-400' : ''}`} />
          <span>Refresh</span>
        </button>
      </div>

      {/* Operation Progress Toast / Banner */}
      {activeOp && (
        <div className={`p-4 rounded-xl border font-mono text-xs shadow-lg transition ${
          activeOp.status === 'ACTIVE'
            ? 'bg-emerald-950/60 border-emerald-500/50 text-emerald-200'
            : activeOp.status === 'FAILED'
            ? 'bg-rose-950/60 border-rose-500/50 text-rose-200'
            : 'bg-blue-950/60 border-blue-500/50 text-blue-200'
        }`}>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <StatusBadge status={activeOp.status} size="sm" />
              <span className="font-bold">Manual Switch: Slot {activeOp.slot} → Node {activeOp.target_node_id}</span>
            </div>
            <button
              onClick={() => setActiveOp(null)}
              className="text-slate-400 hover:text-white text-xs underline"
            >
              Dismiss
            </button>
          </div>
          <div className="mt-2 text-[11px] text-slate-400">
            Operation ID: {activeOp.operation_id}
            {activeOp.error && <div className="text-rose-400 font-semibold mt-1">Error: {activeOp.error}</div>}
          </div>
        </div>
      )}

      {/* Slots Grid */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-5">
        {slotList.map(({ slot, exit, isLocked }) => {
          const statusStr = exit ? exit.status : 'OFFLINE';

          return (
            <div
              key={slot}
              className="glass-panel p-6 rounded-xl border border-slate-800 flex flex-col justify-between space-y-4 glow-cyan"
            >
              {/* Header */}
              <div>
                <div className="flex items-center justify-between pb-3 border-b border-slate-800/80">
                  <div className="flex items-center gap-2">
                    <span className="font-mono text-xs font-bold px-2 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-700/50">
                      SLOT {slot}
                    </span>
                    <span className="text-xs font-mono text-slate-400">tun{slot}</span>
                  </div>
                  <StatusBadge status={statusStr} />
                </div>

                {/* Body Details */}
                <div className="mt-4 space-y-2.5 font-mono text-xs">
                  <div className="flex justify-between">
                    <span className="text-slate-500">Route Table:</span>
                    <span className="text-slate-300 font-bold">{100 + slot}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Fwmark:</span>
                    <span className="text-slate-300 font-bold">{100 + slot}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Node IP:</span>
                    <span className="text-white font-bold">{exit ? exit.ip : 'None'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Country:</span>
                    <span className="text-cyan-300 font-semibold">{exit ? exit.country : '--'}</span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Throughput:</span>
                    <span className="text-emerald-400 font-bold">
                      {exit?.throughput ? `${(exit.throughput / 1_000_000).toFixed(1)} Mbps` : '--'}
                    </span>
                  </div>
                  <div className="flex justify-between">
                    <span className="text-slate-500">Score:</span>
                    <span className="text-amber-400 font-bold">{exit ? exit.score : '--'}</span>
                  </div>
                </div>

                {isLocked && (
                  <div className="mt-4 p-2 rounded bg-amber-950/40 border border-amber-800/40 text-[11px] font-mono text-amber-300 flex items-center gap-2">
                    <Lock className="w-3.5 h-3.5 shrink-0" />
                    <span>Locked by active state lease</span>
                  </div>
                )}
              </div>

              {/* Action Button */}
              <div className="pt-4 border-t border-slate-800/80">
                {isAdmin ? (
                  <button
                    onClick={() => handleStartSwitch(slot)}
                    disabled={isLocked}
                    className="w-full flex items-center justify-center gap-2 py-2 px-3 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-semibold text-xs rounded-lg shadow transition disabled:opacity-40"
                  >
                    <ArrowRightLeft className="w-3.5 h-3.5" />
                    <span>Switch Slot Node</span>
                  </button>
                ) : (
                  <div className="text-center text-[11px] font-mono text-slate-500 italic">
                    Read-only (Admin required to switch)
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Switch Slot Candidate Selector Modal */}
      {switchingSlot !== null && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150">
          <div className="w-full max-w-lg glass-panel p-6 rounded-2xl border border-slate-700 shadow-2xl space-y-4">
            <div className="flex items-center justify-between pb-3 border-b border-slate-800">
              <div className="flex items-center gap-2">
                <ArrowRightLeft className="w-5 h-5 text-cyan-400" />
                <h3 className="text-base font-bold text-white font-mono">
                  MANUAL SWITCH — SLOT {switchingSlot}
                </h3>
              </div>
              <button
                onClick={() => setSwitchingSlot(null)}
                className="text-slate-400 hover:text-white text-xs"
              >
                Cancel
              </button>
            </div>

            <p className="text-xs text-slate-300">
              Select a vetted candidate node. The current tunnel on Slot {switchingSlot} will enter
              graceful <strong className="text-amber-400">DRAINING</strong> (preserving active sockets)
              while the new tunnel initializes and passes fail-closed health verification.
            </p>

            {switchError && (
              <div className="p-3 rounded-lg bg-rose-950/60 border border-rose-800 text-rose-300 text-xs flex items-center gap-2">
                <AlertCircle className="w-4 h-4 shrink-0" />
                <span>{switchError}</span>
              </div>
            )}

            <div>
              <label className="block text-xs font-mono text-slate-400 mb-2">
                Candidate Nodes ({qualifiedNodes.length} Qualified):
              </label>
              <div className="max-h-60 overflow-y-auto space-y-2 pr-1">
                {qualifiedNodes.length === 0 ? (
                  <div className="p-4 text-center text-xs text-slate-500 font-mono">
                    No qualified nodes currently standby.
                  </div>
                ) : (
                  qualifiedNodes.map((n) => (
                    <div
                      key={n.id}
                      onClick={() => setSelectedTargetNode(n.id)}
                      className={`p-3 rounded-lg border font-mono text-xs cursor-pointer transition flex items-center justify-between ${
                        selectedTargetNode === n.id
                          ? 'bg-cyan-950/70 border-cyan-500 text-white'
                          : 'bg-noc-900/80 border-slate-800 text-slate-300 hover:border-slate-700'
                      }`}
                    >
                      <div>
                        <div className="font-bold flex items-center gap-2">
                          <span>{n.ip}</span>
                          <span className="text-[10px] px-1.5 py-0.2 rounded bg-slate-800 text-cyan-300">
                            {n.country}
                          </span>
                        </div>
                        <div className="text-[11px] text-slate-500 mt-0.5">
                          Score: {n.score} • RTT: {n.performance?.rtt_ms || '--'}ms • Speed: {((n.performance?.download_bps || 0) / 1000000).toFixed(1)}M
                        </div>
                      </div>
                      <input
                        type="radio"
                        checked={selectedTargetNode === n.id}
                        onChange={() => setSelectedTargetNode(n.id)}
                        className="text-cyan-500"
                      />
                    </div>
                  ))
                )}
              </div>
            </div>

            <div className="flex gap-3 pt-3 border-t border-slate-800">
              <button
                onClick={handleExecuteSwitch}
                disabled={switchLoading || !selectedTargetNode}
                className="flex-1 py-2.5 px-4 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-semibold text-xs rounded-lg transition disabled:opacity-40"
              >
                {switchLoading ? 'Submitting Switch...' : 'Confirm Switch'}
              </button>
              <button
                onClick={() => setSwitchingSlot(null)}
                className="py-2.5 px-4 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-xs rounded-lg transition"
              >
                Cancel
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};
