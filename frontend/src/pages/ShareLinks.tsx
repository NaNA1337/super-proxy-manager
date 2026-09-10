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
  CheckCircle2,
  FileText,
  FileCode,
  Code2
} from 'lucide-react';
import { CurrentExit, ShareLinkResult, ClientProfile } from '../types';
import { QRCodeModal } from '../components/QRCodeModal';
import { api, getSelectedHostID } from '../api/client';

type ClientFormatTab = 'vless' | 'clash' | 'singbox' | 'xray' | 'socks5';

export const ShareLinks: React.FC = () => {
  const [exits, setExits] = useState<CurrentExit[]>([]);
  const [supportedProtos, setSupportedProtos] = useState<string[]>([]);
  const [allProtos, setAllProtos] = useState<string[]>([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string>('');
  const [selectedProtocol, setSelectedProtocol] = useState<string>('vless');
  const [activeFormatTab, setActiveFormatTab] = useState<ClientFormatTab>('vless');
  const [clientProfiles, setClientProfiles] = useState<ClientProfile[]>([]);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState('');
  const [copiedKey, setCopiedKey] = useState<string | null>(null);

  // QR Modal State
  const [qrData, setQrData] = useState<{ isOpen: boolean; uri: string; title: string; protocol: string }>({
    isOpen: false,
    uri: '',
    title: '',
    protocol: '',
  });

  const fetchData = async () => {
    try {
      const [exitsRes, protoRes] = await Promise.all([
        api.getCurrentExits().catch(() => []),
        api.getShareProtocols().catch(() => ({ supported: ['vless', 'socks5'], all: ['vless', 'socks5', 'vmess', 'trojan', 'shadowsocks', 'http'] })),
      ]);
      setExits(exitsRes || []);
      setSupportedProtos(protoRes.supported || []);
      setAllProtos(protoRes.all || []);
      if (exitsRes && exitsRes.length > 0) {
        setSelectedNodeId(exitsRes[0].node_id);
      }

      // Load client profiles for current host if available
      const hostID = getSelectedHostID();
      if (hostID && hostID !== 'all') {
        const profiles = await api.getHostClientLinks(hostID).catch(() => []);
        setClientProfiles(profiles || []);
      }
    } catch (e) {
      console.error('Failed to load protocols or exits', e);
    }
  };

  useEffect(() => {
    fetchData();
  }, []);

  const handleCopy = async (text: string, key: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedKey(key);
      setTimeout(() => setCopiedKey(null), 2000);
    } catch (e) {
      console.error('Failed to copy', e);
    }
  };

  const handleDownloadFile = (content: string, filename: string, type = 'text/plain;charset=utf-8') => {
    const blob = new Blob([content], { type });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
  };

  const handleCopyAllStructured = async () => {
    const hostID = getSelectedHostID();
    if (!hostID || hostID === 'all') {
      alert('Please select a specific host from the top navbar to export all client links.');
      return;
    }
    setLoading(true);
    try {
      const text = await api.getHostClientLinksAll(hostID);
      await handleCopy(text, 'copy-all-structured');
      alert('All client links (VLESS, Clash Meta, sing-box, Xray, SOCKS5) copied to clipboard!');
    } catch (err: any) {
      alert(`Failed to export all client links: ${err.safeMessage || err.message}`);
    } finally {
      setLoading(false);
    }
  };

  const handleDownloadAllStructured = async () => {
    const hostID = getSelectedHostID();
    if (!hostID || hostID === 'all') {
      alert('Please select a specific host from the top navbar to export all client links.');
      return;
    }
    setLoading(true);
    try {
      const text = await api.getHostClientLinksAll(hostID);
      handleDownloadFile(text, `super-proxy-clients-${hostID}-${new Date().toISOString().slice(0, 10)}.txt`);
    } catch (err: any) {
      alert(`Failed to export: ${err.safeMessage || err.message}`);
    } finally {
      setLoading(false);
    }
  };

  // Find active profile for selected node
  const activeProfile = clientProfiles.find((p) => p.node_id === selectedNodeId) || clientProfiles[0];

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide font-mono flex items-center gap-2">
            <Share2 className="w-5 h-5 text-cyan-400" />
            CLIENT SHARELINKS & MULTI-FORMAT EXPORTERS
          </h1>
          <p className="text-xs text-slate-400 font-mono mt-1">
            Read from active daemon runtime. Pure client configurations for VLESS Reality, Clash Meta, sing-box, and Xray.
          </p>
        </div>

        <div className="flex items-center gap-2">
          <button
            onClick={handleCopyAllStructured}
            disabled={loading}
            className="px-3.5 py-2 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-mono text-xs font-bold rounded-lg shadow-lg shadow-cyan-950 transition flex items-center gap-1.5"
          >
            <Copy className="w-3.5 h-3.5" />
            {copiedKey === 'copy-all-structured' ? 'Copied All!' : 'Copy All Client Links'}
          </button>
          <button
            onClick={handleDownloadAllStructured}
            disabled={loading}
            className="px-3 py-2 bg-slate-900 border border-slate-700 hover:bg-slate-800 text-slate-300 font-mono text-xs rounded-lg transition flex items-center gap-1.5"
            title="Download formatted text file"
          >
            <Download className="w-3.5 h-3.5" />
            Download
          </button>
        </div>
      </div>

      {/* Protocol Support Status Grid */}
      <div className="p-4 glass-panel rounded-xl border border-slate-800 space-y-2 font-mono text-xs">
        <span className="text-[10px] text-slate-400 uppercase tracking-wider font-bold">
          DAEMON RUNTIME PROTOCOL STATUS
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
                  {isSupported ? 'SUPPORTED' : 'NOT SUPPORTED'}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Multi-Client Format Explorer */}
      <div className="glass-panel p-6 rounded-xl border border-slate-800 space-y-5">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3 pb-3 border-b border-slate-800">
          <div className="flex items-center gap-3">
            <label className="text-xs font-mono text-slate-400 uppercase font-bold">Target Exit Node:</label>
            <select
              value={selectedNodeId}
              onChange={(e) => setSelectedNodeId(e.target.value)}
              className="px-3 py-1.5 bg-noc-900 border border-slate-700 rounded-lg text-xs font-mono text-white focus:outline-none focus:border-cyan-500"
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

          {/* Client Tabs */}
          <div className="flex items-center gap-1.5 bg-slate-900/80 p-1 rounded-lg border border-slate-800 font-mono text-xs">
            <button
              onClick={() => setActiveFormatTab('vless')}
              className={`px-3 py-1.5 rounded-md font-bold transition ${
                activeFormatTab === 'vless'
                  ? 'bg-cyan-600 text-white shadow'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              VLESS URI
            </button>
            <button
              onClick={() => setActiveFormatTab('clash')}
              className={`px-3 py-1.5 rounded-md font-bold transition ${
                activeFormatTab === 'clash'
                  ? 'bg-cyan-600 text-white shadow'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Clash Meta
            </button>
            <button
              onClick={() => setActiveFormatTab('singbox')}
              className={`px-3 py-1.5 rounded-md font-bold transition ${
                activeFormatTab === 'singbox'
                  ? 'bg-cyan-600 text-white shadow'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              sing-box
            </button>
            <button
              onClick={() => setActiveFormatTab('xray')}
              className={`px-3 py-1.5 rounded-md font-bold transition ${
                activeFormatTab === 'xray'
                  ? 'bg-cyan-600 text-white shadow'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Xray JSON
            </button>
            <button
              onClick={() => setActiveFormatTab('socks5')}
              className={`px-3 py-1.5 rounded-md font-bold transition ${
                activeFormatTab === 'socks5'
                  ? 'bg-cyan-600 text-white shadow'
                  : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              SOCKS5
            </button>
          </div>
        </div>

        {/* Tab Content Display */}
        {activeProfile ? (
          <div className="space-y-4 font-mono text-xs">
            {activeFormatTab === 'vless' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-slate-400">
                  <span className="font-bold text-white">Standard VLESS Reality Link (v2rayN, v2rayNG, NekoBox)</span>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleCopy(activeProfile.uri, 'vless-uri')}
                      className="px-3 py-1.5 rounded bg-cyan-600 hover:bg-cyan-500 text-white font-bold transition flex items-center gap-1.5"
                    >
                      {copiedKey === 'vless-uri' ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>Copy URI</span>
                    </button>
                    <button
                      onClick={() =>
                        setQrData({
                          isOpen: true,
                          uri: activeProfile.uri,
                          title: activeProfile.name,
                          protocol: 'VLESS Reality',
                        })
                      }
                      className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 font-bold transition flex items-center gap-1.5"
                    >
                      <QrCode className="w-3.5 h-3.5" />
                      <span>QR Code</span>
                    </button>
                  </div>
                </div>
                <div className="p-4 bg-noc-950 rounded-xl border border-slate-800 text-slate-300 break-all select-all leading-relaxed">
                  {activeProfile.uri}
                </div>
              </div>
            )}

            {activeFormatTab === 'clash' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-slate-400">
                  <span className="font-bold text-white">Clash Meta / Mihomo Proxy Snippet (YAML)</span>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleCopy(activeProfile.clash_config || '', 'clash-yaml')}
                      className="px-3 py-1.5 rounded bg-cyan-600 hover:bg-cyan-500 text-white font-bold transition flex items-center gap-1.5"
                    >
                      {copiedKey === 'clash-yaml' ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>Copy YAML</span>
                    </button>
                    <button
                      onClick={() => handleDownloadFile(activeProfile.clash_config || '', `${activeProfile.name}-clash.yaml`)}
                      className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 font-bold transition flex items-center gap-1.5"
                    >
                      <Download className="w-3.5 h-3.5" />
                      <span>Download</span>
                    </button>
                  </div>
                </div>
                <pre className="p-4 bg-noc-950 rounded-xl border border-slate-800 text-cyan-300 overflow-x-auto leading-relaxed">
                  {activeProfile.clash_config || '# Not supported'}
                </pre>
              </div>
            )}

            {activeFormatTab === 'singbox' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-slate-400">
                  <span className="font-bold text-white">sing-box Outbound Object (JSON)</span>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleCopy(JSON.stringify(activeProfile.singbox_config, null, 2), 'singbox-json')}
                      className="px-3 py-1.5 rounded bg-cyan-600 hover:bg-cyan-500 text-white font-bold transition flex items-center gap-1.5"
                    >
                      {copiedKey === 'singbox-json' ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>Copy JSON</span>
                    </button>
                    <button
                      onClick={() => handleDownloadFile(JSON.stringify(activeProfile.singbox_config, null, 2), `${activeProfile.name}-singbox.json`)}
                      className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 font-bold transition flex items-center gap-1.5"
                    >
                      <Download className="w-3.5 h-3.5" />
                      <span>Download</span>
                    </button>
                  </div>
                </div>
                <pre className="p-4 bg-noc-950 rounded-xl border border-slate-800 text-amber-300 overflow-x-auto leading-relaxed">
                  {JSON.stringify(activeProfile.singbox_config, null, 2)}
                </pre>
              </div>
            )}

            {activeFormatTab === 'xray' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-slate-400">
                  <span className="font-bold text-white">Xray-core Outbound Object (JSON)</span>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleCopy(JSON.stringify(activeProfile.xray_config, null, 2), 'xray-json')}
                      className="px-3 py-1.5 rounded bg-cyan-600 hover:bg-cyan-500 text-white font-bold transition flex items-center gap-1.5"
                    >
                      {copiedKey === 'xray-json' ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>Copy JSON</span>
                    </button>
                    <button
                      onClick={() => handleDownloadFile(JSON.stringify(activeProfile.xray_config, null, 2), `${activeProfile.name}-xray.json`)}
                      className="px-3 py-1.5 rounded bg-slate-800 hover:bg-slate-700 text-slate-200 font-bold transition flex items-center gap-1.5"
                    >
                      <Download className="w-3.5 h-3.5" />
                      <span>Download</span>
                    </button>
                  </div>
                </div>
                <pre className="p-4 bg-noc-950 rounded-xl border border-slate-800 text-blue-300 overflow-x-auto leading-relaxed">
                  {JSON.stringify(activeProfile.xray_config, null, 2)}
                </pre>
              </div>
            )}

            {activeFormatTab === 'socks5' && (
              <div className="space-y-3">
                <div className="flex items-center justify-between text-slate-400">
                  <span className="font-bold text-white">Direct SOCKS5 Inbound URI</span>
                  <div className="flex items-center gap-2">
                    <button
                      onClick={() => handleCopy(activeProfile.uri, 'socks5-uri')}
                      className="px-3 py-1.5 rounded bg-cyan-600 hover:bg-cyan-500 text-white font-bold transition flex items-center gap-1.5"
                    >
                      {copiedKey === 'socks5-uri' ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                      <span>Copy URI</span>
                    </button>
                  </div>
                </div>
                <div className="p-4 bg-noc-950 rounded-xl border border-slate-800 text-slate-300 break-all select-all">
                  {activeProfile.uri}
                </div>
              </div>
            )}
          </div>
        ) : (
          <div className="py-8 text-center text-slate-500 font-mono text-xs space-y-2">
            <Radio className="w-6 h-6 mx-auto text-slate-600 animate-pulse" />
            <p>Select a host and node to inspect live exported client links.</p>
          </div>
        )}
      </div>

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
