import React, { useState, useEffect } from 'react';
import {
  Share2,
  Copy,
  Check,
  QrCode,
  Layers,
  Globe,
  Radio,
  AlertCircle,
  Download,
  CheckCircle2
} from 'lucide-react';
import { CurrentExit, ShareLinkResult } from '../types';
import { QRCodeModal } from '../components/QRCodeModal';
import { api } from '../api/client';

export const ShareLinks: React.FC = () => {
  const [exits, setExits] = useState<CurrentExit[]>([]);
  const [supportedProtos, setSupportedProtos] = useState<string[]>([]);
  const [allProtos, setAllProtos] = useState<string[]>([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string>('');
  const [selectedProtocol, setSelectedProtocol] = useState<string>('socks5');
  const [generatedResult, setGeneratedResult] = useState<ShareLinkResult | null>(null);
  const [batchResults, setBatchResults] = useState<ShareLinkResult[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [copiedLink, setCopiedLink] = useState<string | null>(null);

  // QR Modal State
  const [qrData, setQrData] = useState<{ isOpen: boolean; uri: string; title: string; protocol: string }>({
    isOpen: false,
    uri: '',
    title: '',
    protocol: '',
  });

  useEffect(() => {
    const fetchData = async () => {
      try {
        const [exitsRes, protoRes] = await Promise.all([
          api.getCurrentExits().catch(() => []),
          api.getShareProtocols().catch(() => ({ supported: ['socks5'], all: ['socks5', 'vless'] })),
        ]);
        setExits(exitsRes || []);
        setSupportedProtos(protoRes.supported || []);
        setAllProtos(protoRes.all || []);
        if (exitsRes && exitsRes.length > 0) {
          setSelectedNodeId(exitsRes[0].node_id);
        }
      } catch (e) {
        console.error('Failed to load protocols or exits', e);
      }
    };
    fetchData();
  }, []);

  const handleGenerate = async () => {
    if (!selectedNodeId || !selectedProtocol) return;
    setLoading(true);
    setError('');
    setGeneratedResult(null);

    try {
      const res = await api.generateShareLink(selectedNodeId, selectedProtocol);
      setGeneratedResult(res);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Generation failed');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleBatchGenerate = async () => {
    setLoading(true);
    setError('');
    try {
      const res = await api.batchGenerateShareLinks(supportedProtos);
      setBatchResults(res || []);
    } catch (err: unknown) {
      if (err instanceof Error) {
        setError(err.message);
      } else {
        setError('Batch generation failed');
      }
    } finally {
      setLoading(false);
    }
  };

  const handleCopy = async (uri: string) => {
    try {
      await navigator.clipboard.writeText(uri);
      setCopiedLink(uri);
      setTimeout(() => setCopiedLink(null), 2000);
    } catch (e) {
      console.error('Failed to copy', e);
    }
  };

  const handleDownloadAll = () => {
    const activeURIs = batchResults.filter((r) => r.supported && r.uri).map((r) => r.uri).join('\n');
    const blob = new Blob([activeURIs], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `super-proxy-sharelinks-${new Date().toISOString().slice(0, 10)}.txt`;
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-white tracking-wide">CLIENT SHARELINK GENERATOR</h1>
        <p className="text-xs text-slate-400">
          Export standards-compliant proxy connection links (VLESS Reality, SOCKS5, VMess) with pure client-side QR generation
        </p>
      </div>

      {/* Protocol Support Status Grid */}
      <div className="p-4 glass-panel rounded-xl border border-slate-800 space-y-2 font-mono text-xs">
        <span className="text-[10px] text-slate-400 uppercase tracking-wider font-bold">
          BACKEND RUNTIME PROTOCOL VERIFICATION
        </span>
        <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-6 gap-2 pt-1">
          {allProtos.map((p) => {
            const isSupported = supportedProtos.includes(p);
            return (
              <div
                key={p}
                className={`p-2.5 rounded-lg border flex items-center justify-between ${
                  isSupported
                    ? 'bg-emerald-950/40 border-emerald-500/50 text-emerald-300'
                    : 'bg-slate-900/50 border-slate-800 text-slate-500'
                }`}
              >
                <span className="font-bold uppercase text-[11px]">{p}</span>
                <span
                  className={`text-[9px] px-1.5 py-0.5 rounded font-bold uppercase ${
                    isSupported ? 'bg-emerald-900 text-emerald-200' : 'bg-slate-800 text-slate-400'
                  }`}
                >
                  {isSupported ? 'ACTIVE' : 'NOT CONFIGURED'}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Generator Control Card */}
      <div className="glass-panel p-6 rounded-xl border border-slate-800 space-y-5">
        <h3 className="text-sm font-bold text-white font-mono uppercase tracking-wider">
          TARGET NODE & PROTOCOL SELECTOR
        </h3>

        {error && (
          <div className="p-3 rounded-lg bg-rose-950/60 border border-rose-800 text-rose-300 text-xs flex items-center gap-2">
            <AlertCircle className="w-4 h-4 shrink-0 text-rose-400" />
            <span>{error}</span>
          </div>
        )}

        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          {/* Node Selector */}
          <div>
            <label className="block text-xs font-mono text-slate-400 mb-2">Active Egress Slot / Node:</label>
            <select
              value={selectedNodeId}
              onChange={(e) => setSelectedNodeId(e.target.value)}
              className="w-full px-3 py-2.5 bg-noc-900 border border-slate-700 rounded-lg text-xs font-mono text-white focus:outline-none focus:border-cyan-500"
            >
              {exits.length === 0 ? (
                <option value="">No active egress nodes</option>
              ) : (
                exits.map((exit) => (
                  <option key={exit.node_id} value={exit.node_id}>
                    Slot {exit.slot}: {exit.ip} ({exit.country}) • {((exit.throughput || 0) / 1000000).toFixed(1)} Mbps
                  </option>
                ))
              )}
            </select>
          </div>

          {/* Protocol Selector */}
          <div>
            <label className="block text-xs font-mono text-slate-400 mb-2">Client Proxy Protocol:</label>
            <select
              value={selectedProtocol}
              onChange={(e) => setSelectedProtocol(e.target.value)}
              className="w-full px-3 py-2.5 bg-noc-900 border border-slate-700 rounded-lg text-xs font-mono text-white focus:outline-none focus:border-cyan-500"
            >
              {allProtos.map((p) => {
                const supported = supportedProtos.includes(p);
                return (
                  <option key={p} value={p}>
                    {p.toUpperCase()} {supported ? '(Backend Ready)' : '(Not Active in Xray)'}
                  </option>
                );
              })}
            </select>
          </div>
        </div>

        <div className="flex flex-wrap gap-3 pt-2">
          <button
            onClick={handleGenerate}
            disabled={loading || !selectedNodeId}
            className="px-5 py-2.5 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-semibold text-xs rounded-lg shadow-lg shadow-cyan-950/40 transition disabled:opacity-40"
          >
            {loading ? 'Generating...' : 'Generate ShareLink'}
          </button>

          <button
            onClick={handleBatchGenerate}
            disabled={loading || exits.length === 0}
            className="px-4 py-2.5 bg-slate-800 hover:bg-slate-700 text-slate-300 font-medium text-xs rounded-lg transition disabled:opacity-40"
          >
            Batch Generate All Active Nodes
          </button>
        </div>
      </div>

      {/* Generated Result Card */}
      {generatedResult && (
        <div className="glass-panel p-5 rounded-xl border border-cyan-500/40 glow-cyan space-y-3 font-mono text-xs">
          <div className="flex items-center justify-between pb-2 border-b border-slate-800">
            <span className="font-bold text-white uppercase flex items-center gap-2">
              <span className="text-cyan-400 font-bold">[{generatedResult.protocol.toUpperCase()}]</span>
              <span>{generatedResult.description}</span>
            </span>
            <span
              className={`text-[10px] px-2 py-0.5 rounded font-bold ${
                generatedResult.supported
                  ? 'bg-emerald-950 text-emerald-300 border border-emerald-500/50'
                  : 'bg-rose-950 text-rose-300 border border-rose-800'
              }`}
            >
              {generatedResult.supported ? 'VALIDATED' : 'UNSUPPORTED'}
            </span>
          </div>

          {generatedResult.uri ? (
            <>
              <div className="p-3 bg-noc-950 rounded-lg border border-slate-800 text-slate-300 break-all select-all">
                {generatedResult.uri}
              </div>

              <div className="flex gap-3 pt-1">
                <button
                  onClick={() => handleCopy(generatedResult.uri!)}
                  className="flex items-center gap-1.5 px-3 py-1.5 bg-cyan-600 hover:bg-cyan-500 text-white font-semibold rounded-lg transition"
                >
                  {copiedLink === generatedResult.uri ? (
                    <>
                      <Check className="w-3.5 h-3.5" /> Copied!
                    </>
                  ) : (
                    <>
                      <Copy className="w-3.5 h-3.5" /> Copy Link
                    </>
                  )}
                </button>

                <button
                  onClick={() =>
                    setQrData({
                      isOpen: true,
                      uri: generatedResult.uri!,
                      title: `${generatedResult.country} Exit`,
                      protocol: generatedResult.protocol,
                    })
                  }
                  className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 font-semibold rounded-lg transition"
                >
                  <QrCode className="w-3.5 h-3.5" /> View QR Code
                </button>
              </div>
            </>
          ) : (
            <div className="p-3 rounded-lg bg-amber-950/40 border border-amber-800/40 text-amber-300">
              {generatedResult.error || 'Protocol cannot be generated with current backend settings.'}
            </div>
          )}
        </div>
      )}

      {/* Batch Results Table */}
      {batchResults.length > 0 && (
        <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-4">
          <div className="flex items-center justify-between">
            <h3 className="text-sm font-bold text-white font-mono uppercase tracking-wider">
              BATCH GENERATED CLIENT PROXIES ({batchResults.length})
            </h3>
            <button
              onClick={handleDownloadAll}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-cyan-400 font-mono text-xs rounded-lg transition"
            >
              <Download className="w-3.5 h-3.5" />
              <span>Download Text File</span>
            </button>
          </div>

          <div className="space-y-2">
            {batchResults.map((item, idx) => (
              <div
                key={idx}
                className="p-3 rounded-lg bg-noc-900 border border-slate-800 font-mono text-xs flex flex-col sm:flex-row sm:items-center justify-between gap-3"
              >
                <div>
                  <div className="flex items-center gap-2 font-bold text-white">
                    <span className="text-cyan-400">[{item.protocol.toUpperCase()}]</span>
                    <span>{item.country}</span>
                    <span className="text-slate-400 font-normal">({item.node_id})</span>
                  </div>
                  {item.uri ? (
                    <div className="text-[11px] text-slate-400 truncate max-w-xl mt-1">
                      {item.uri}
                    </div>
                  ) : (
                    <div className="text-[11px] text-amber-400 mt-1">
                      {item.error || 'Not supported by backend runtime'}
                    </div>
                  )}
                </div>

                {item.uri && (
                  <div className="flex items-center gap-2 shrink-0">
                    <button
                      onClick={() => handleCopy(item.uri!)}
                      className="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded transition"
                      title="Copy"
                    >
                      {copiedLink === item.uri ? <Check className="w-4 h-4 text-emerald-400" /> : <Copy className="w-4 h-4" />}
                    </button>
                    <button
                      onClick={() =>
                        setQrData({
                          isOpen: true,
                          uri: item.uri!,
                          title: `${item.country} (${item.protocol.toUpperCase()})`,
                          protocol: item.protocol,
                        })
                      }
                      className="p-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded transition"
                      title="QR Code"
                    >
                      <QrCode className="w-4 h-4" />
                    </button>
                  </div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* QR Code Modal */}
      <QRCodeModal
        isOpen={qrData.isOpen}
        onClose={() => setQrData({ ...qrData, isOpen: false })}
        uri={qrData.uri}
        title={qrData.title}
        protocol={qrData.protocol}
      />
    </div>
  );
};
