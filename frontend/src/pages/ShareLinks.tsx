import React, { useState, useEffect } from 'react';
import {
  Share2,
  Copy,
  Check,
  QrCode,
  Download,
  Layers,
  Globe,
  Radio,
  AlertTriangle,
  Server,
  Archive,
  RefreshCw,
  FileText,
  CheckSquare,
  Square,
  Lock,
  X,
  ChevronRight,
  ChevronDown
} from 'lucide-react';
import { Host, NodeClientConfig, CanonicalProfile } from '../types';
import { QRCodeModal } from '../components/QRCodeModal';
import { api, getSelectedHostID, setSelectedHostID } from '../api/client';

interface ShareLinksProps {
  selectedHostID?: string;
  onSelectHost?: (hostID: string) => void;
}

export const ShareLinks: React.FC<ShareLinksProps> = ({ selectedHostID, onSelectHost }) => {
  const [hosts, setHosts] = useState<Host[]>([]);
  const [selectedHost, setSelectedHost] = useState<Host | null>(null);
  const [nodes, setNodes] = useState<{ id: string; ip: string; country: string }[]>([]);
  const [selectedNodeId, setSelectedNodeId] = useState<string>('');
  const [nodeConfig, setNodeConfig] = useState<NodeClientConfig | null>(null);

  const [loading, setLoading] = useState(false);
  const [loadingConfig, setLoadingConfig] = useState(false);
  const [copiedKey, setCopiedKey] = useState<string | null>(null);
  const [viewMode, setViewMode] = useState<'single' | 'all'>('single');

  // All Hosts hierarchical state
  const [allHostsData, setAllHostsData] = useState<{ host: Host; nodes: { id: string; ip: string; country: string }[] }[]>([]);
  const [expandedHosts, setExpandedHosts] = useState<Record<string, boolean>>({});

  // Batch Export Modal State
  const [isExportModalOpen, setIsExportModalOpen] = useState(false);
  const [selectedForExport, setSelectedForExport] = useState<Record<string, boolean>>({});
  const [exportingZip, setExportingZip] = useState(false);

  // QR Modal State
  const [qrData, setQrData] = useState<{ isOpen: boolean; uri: string; title: string; protocol: string }>({
    isOpen: false,
    uri: '',
    title: '',
    protocol: '',
  });

  // Load Host Registry
  const loadHosts = async () => {
    setLoading(true);
    try {
      const hostList = await api.listHosts().catch(() => []);
      setHosts(hostList);

      if (hostList.length > 0) {
        const savedHostID = getSelectedHostID();
        const found = hostList.find((h) => h.id === savedHostID) || hostList.find((h) => h.is_default) || hostList[0];
        setSelectedHost(found);
        setSelectedHostID(found.id);
      }
    } catch (e) {
      console.error('Failed to load hosts', e);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadHosts();
  }, []);

  useEffect(() => {
    if (selectedHostID && hosts.length > 0) {
      const found = hosts.find((h) => h.id === selectedHostID);
      if (found && found.id !== selectedHost?.id) {
        setSelectedHost(found);
      }
    }
  }, [selectedHostID, hosts]);

  // When selected host changes, fetch its node list from daemon
  useEffect(() => {
    if (!selectedHost) return;

    const fetchHostNodes = async () => {
      try {
        // Fetch exits from selected host daemon
        const exits = await api.getCurrentExits().catch(() => []);
        let candidateNodes: { id: string; ip: string; country: string }[] = [];

        if (exits && exits.length > 0) {
          candidateNodes = exits.map((e) => ({
            id: e.node_id,
            ip: e.ip,
            country: e.country || 'GLOBAL',
          }));
        } else {
          // Fallback to pool qualified if exits is empty
          const qualified = await api.getPoolQualified().catch(() => []);
          candidateNodes = (qualified || []).map((q) => ({
            id: q.id,
            ip: q.ip,
            country: q.country || 'GLOBAL',
          }));
        }

        if (candidateNodes.length === 0) {
          const bundle = await api.getHostClientConfig(selectedHost.id);
          candidateNodes = (bundle.nodes || []).map(n => ({
            id: n.node_id, ip: n.endpoint_address || '', country: n.country || 'GLOBAL',
          }));
        }
        setNodes(candidateNodes);
        if (candidateNodes.length > 0) {
          setSelectedNodeId(candidateNodes[0].id);
        } else {
          setSelectedNodeId('');
          setNodeConfig(null);
        }
      } catch (err) {
        console.error('Failed to fetch host nodes', err);
        setNodes([]);
        setSelectedNodeId('');
      }
    };

    fetchHostNodes();
  }, [selectedHost?.id]);

  // When selected host or node changes, fetch canonical client config from Manager API
  useEffect(() => {
    if (!selectedHost || !selectedNodeId) return;

    const fetchConfig = async () => {
      setLoadingConfig(true);
      try {
        const resp = await api.getHostClientConfig(selectedHost.id, selectedNodeId);
        if (resp && resp.nodes && resp.nodes.length > 0) {
          setNodeConfig(resp.nodes[0]);
        } else if (resp && resp.profiles && resp.profiles.length > 0) {
          setNodeConfig({
            available: resp.available,
            error: resp.error,
            node_id: selectedNodeId,
            host_id: selectedHost.id,
            host_name: selectedHost.name,
            profiles: resp.profiles,
          });
        } else {
          setNodeConfig({
            available: resp?.available ?? false,
            error: resp?.error || 'Client configuration temporarily unavailable (Xray unavailable)',
            node_id: selectedNodeId,
            host_id: selectedHost.id,
            host_name: selectedHost.name,
            profiles: [],
          });
        }
      } catch (err: any) {
        console.error('Failed to fetch canonical client config', err);
        setNodeConfig({
          available: false,
          error: err.safeMessage || err.message || 'runtime endpoint unavailable',
          node_id: selectedNodeId,
          host_id: selectedHost.id,
          host_name: selectedHost.name,
          profiles: [],
        });
      } finally {
        setLoadingConfig(false);
      }
    };

    fetchConfig();
  }, [selectedHost?.id, selectedNodeId]);

  // Fetch All Hosts and Nodes for All Hosts mode
  const fetchAllHostsOverview = async () => {
    try {
      const hostList = await api.listHosts().catch(() => []);
      const overview: { host: Host; nodes: { id: string; ip: string; country: string }[] }[] = [];

      for (const h of hostList) {
        // Fetch nodes using host-specific header
        setSelectedHostID(h.id);
        const exits = await api.getCurrentExits().catch(() => []);
        overview.push({
          host: h,
          nodes: exits.map((e) => ({ id: e.node_id, ip: e.ip, country: e.country || 'GLOBAL' })),
        });
      }

      // Restore active host ID
      if (selectedHost) {
        setSelectedHostID(selectedHost.id);
      }

      setAllHostsData(overview);
    } catch (e) {
      console.error('Failed to load all hosts overview', e);
    }
  };

  useEffect(() => {
    if (viewMode === 'all') {
      fetchAllHostsOverview();
    }
  }, [viewMode]);

  // Copy handler with audit
  const handleCopyProfile = async (profile: CanonicalProfile) => {
    try {
      await navigator.clipboard.writeText(profile.content);
      setCopiedKey(profile.id);
      setTimeout(() => setCopiedKey(null), 2000);

      // Audit log (zero secret leakage)
      if (selectedHost && selectedNodeId) {
        api.auditClientConfig(selectedHost.id, selectedNodeId, 'copy', profile.id).catch(() => {});
      }
    } catch (e) {
      console.error('Failed to copy', e);
    }
  };

  // Download handler with canonical filename and MIME type
  const handleDownloadProfile = (profile: CanonicalProfile) => {
    const filename = profile.filename || `${profile.id}.txt`;
    const mimeType = profile.mime_type || 'text/plain;charset=utf-8';
    const blob = new Blob([profile.content], { type: mimeType });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    URL.revokeObjectURL(url);

    // Audit download
    if (selectedHost && selectedNodeId) {
      api.auditClientConfig(selectedHost.id, selectedNodeId, 'download', profile.id).catch(() => {});
    }
  };

  // Copy All canonical profiles with formatted delimiter
  const handleCopyAll = async () => {
    if (!nodeConfig || !nodeConfig.available || nodeConfig.profiles.length === 0) return;

    const sections: string[] = [];
    for (const p of nodeConfig.profiles) {
      sections.push(`=== ${p.name} ===\n${p.content}`);
    }

    const fullText = sections.join('\n\n');
    try {
      await navigator.clipboard.writeText(fullText);
      setCopiedKey('copy-all');
      setTimeout(() => setCopiedKey(null), 2000);

      if (selectedHost && selectedNodeId) {
        api.auditClientConfig(selectedHost.id, selectedNodeId, 'copy_all', 'all_profiles').catch(() => {});
      }
    } catch (e) {
      console.error('Failed to copy all', e);
    }
  };

  // Open QR Code modal
  const handleOpenQR = (profile: CanonicalProfile) => {
    setQrData({
      isOpen: true,
      uri: profile.content,
      title: `${selectedHost?.name} / ${selectedNodeId} - ${profile.name}`,
      protocol: profile.id,
    });
    if (selectedHost && selectedNodeId) {
      api.auditClientConfig(selectedHost.id, selectedNodeId, 'qr', profile.id).catch(() => {});
    }
  };

  // Batch Export ZIP
  const handleBatchExport = async () => {
    const selections: { host_id: string; node_id: string }[] = [];
    for (const key of Object.keys(selectedForExport)) {
      if (selectedForExport[key]) {
        const [hId, nId] = key.split(':::');
        if (hId && nId) {
          selections.push({ host_id: hId, node_id: nId });
        }
      }
    }

    if (selections.length === 0) {
      alert('Please select at least one node to export.');
      return;
    }

    setExportingZip(true);
    try {
      const blob = await api.exportClientConfigsZip(selections);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `super-proxy-configs-${new Date().toISOString().slice(0, 10)}.zip`;
      a.click();
      URL.revokeObjectURL(url);
      setIsExportModalOpen(false);
    } catch (err: any) {
      alert(`Export failed: ${err.message}`);
    } finally {
      setExportingZip(false);
    }
  };

  const getStatusBadge = (status: string) => {
    const s = (status || '').toLowerCase();
    if (s === 'online' || s === 'healthy') {
      return (
        <span className="flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold bg-emerald-950/80 text-emerald-400 border border-emerald-800">
          <span className="w-1.5 h-1.5 rounded-full bg-emerald-400 animate-pulse" />
          ONLINE
        </span>
      );
    }
    if (s === 'degraded') {
      return (
        <span className="flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold bg-amber-950/80 text-amber-400 border border-amber-800">
          <span className="w-1.5 h-1.5 rounded-full bg-amber-400" />
          DEGRADED
        </span>
      );
    }
    if (s === 'offline') {
      return (
        <span className="flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold bg-rose-950/80 text-rose-400 border border-rose-800">
          <span className="w-1.5 h-1.5 rounded-full bg-rose-400" />
          OFFLINE
        </span>
      );
    }
    return (
      <span className="flex items-center gap-1.5 px-2 py-0.5 rounded-full text-[10px] font-bold bg-slate-800 text-slate-400 border border-slate-700">
        <span className="w-1.5 h-1.5 rounded-full bg-slate-400" />
        UNKNOWN
      </span>
    );
  };

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
        <div>
          <h1 className="text-xl font-bold text-white tracking-wide font-mono flex items-center gap-2">
            <Share2 className="w-5 h-5 text-cyan-400" />
            MULTI-HOST CLIENT CONFIGURATION CENTER
          </h1>
          <p className="text-xs text-slate-400 font-mono mt-1">
            Canonical client profiles served directly from super-proxy daemons (VLESS, Clash Meta, sing-box, Xray, Subscription).
          </p>
        </div>

        <div className="flex items-center gap-2">
          {/* View Mode Toggle */}
          <div className="flex bg-slate-900 border border-slate-800 rounded-lg p-0.5 font-mono text-xs">
            <button
              onClick={() => setViewMode('single')}
              className={`px-3 py-1.5 rounded-md font-semibold transition ${
                viewMode === 'single' ? 'bg-cyan-600 text-white shadow' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              Selected Host
            </button>
            <button
              onClick={() => setViewMode('all')}
              className={`px-3 py-1.5 rounded-md font-semibold transition ${
                viewMode === 'all' ? 'bg-cyan-600 text-white shadow' : 'text-slate-400 hover:text-slate-200'
              }`}
            >
              All Hosts View
            </button>
          </div>

          {/* Batch Export Button */}
          <button
            onClick={() => {
              // Pre-populate export selection with current host/node
              if (selectedHost && selectedNodeId) {
                setSelectedForExport({ [`${selectedHost.id}:::${selectedNodeId}`]: true });
              }
              fetchAllHostsOverview();
              setIsExportModalOpen(true);
            }}
            className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-cyan-400 font-mono text-xs font-bold rounded-lg border border-slate-700 transition shadow-sm"
          >
            <Archive className="w-3.5 h-3.5" />
            Batch Export ZIP
          </button>
        </div>
      </div>

      {viewMode === 'single' ? (
        <>
          {/* Host & Node Selectors Bar */}
          <div className="glass-panel p-4 rounded-xl border border-slate-800 font-mono text-xs flex flex-wrap items-center justify-between gap-4">
            <div className="flex flex-wrap items-center gap-4">
              {/* Host Selector */}
              <div className="flex items-center gap-2">
                <span className="text-slate-400 uppercase font-bold flex items-center gap-1.5">
                  <Server className="w-3.5 h-3.5 text-cyan-400" />
                  Host:
                </span>
                <select
                  value={selectedHost?.id || ''}
                  onChange={(e) => {
                    const found = hosts.find((h) => h.id === e.target.value);
                    if (found) {
                      setSelectedHost(found);
                      setSelectedHostID(found.id);
                    }
                  }}
                  className="px-3 py-1.5 bg-slate-900 border border-slate-700 rounded-lg text-xs font-bold text-white focus:outline-none focus:border-cyan-500"
                >
                  {hosts.map((h) => (
                    <option key={h.id} value={h.id}>
                      {h.name} ({h.region || h.address}) {h.is_default ? '★' : ''}
                    </option>
                  ))}
                </select>
                {selectedHost && getStatusBadge(selectedHost.status)}
              </div>

              {/* Node Selector */}
              <div className="flex items-center gap-2">
                <span className="text-slate-400 uppercase font-bold flex items-center gap-1.5">
                  <Globe className="w-3.5 h-3.5 text-emerald-400" />
                  Node:
                </span>
                <select
                  value={selectedNodeId}
                  onChange={(e) => setSelectedNodeId(e.target.value)}
                  className="px-3 py-1.5 bg-slate-900 border border-slate-700 rounded-lg text-xs font-bold text-white focus:outline-none focus:border-cyan-500"
                >
                  {nodes.length === 0 ? (
                    <option value="">No active egress nodes</option>
                  ) : (
                    nodes.map((n) => (
                      <option key={n.id} value={n.id}>
                        {n.id} ({n.country}) • {n.ip}
                      </option>
                    ))
                  )}
                </select>
              </div>
            </div>

            {/* Quick Actions */}
            <div className="flex items-center gap-2">
              <button
                onClick={handleCopyAll}
                disabled={!nodeConfig?.available || loadingConfig}
                className={`px-3 py-1.5 rounded-lg font-bold transition flex items-center gap-1.5 ${
                  nodeConfig?.available && !loadingConfig
                    ? 'bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white shadow-md shadow-cyan-950'
                    : 'bg-slate-800 text-slate-500 cursor-not-allowed border border-slate-700'
                }`}
              >
                <Copy className="w-3.5 h-3.5" />
                {copiedKey === 'copy-all' ? 'Copied All Profiles!' : 'Copy All Profiles'}
              </button>
            </div>
          </div>

          {/* Runtime Unavailable Warning Banner */}
          {nodeConfig && (!nodeConfig.available || nodeConfig.error) && (
            <div className="p-4 rounded-xl bg-rose-950/30 border border-rose-500/50 font-mono text-xs text-rose-200 flex items-start gap-3 shadow-lg">
              <AlertTriangle className="w-5 h-5 text-rose-400 shrink-0 mt-0.5" />
              <div className="space-y-1">
                <div className="font-bold text-rose-300 tracking-wide uppercase flex items-center gap-2">
                  <span>Xray Unavailable / Client Configuration Temporarily Unavailable</span>
                </div>
                <p className="text-rose-200/80">
                  {nodeConfig.error || 'The super-proxy daemon runtime endpoint is not available or Xray has stopped. Client links cannot be copied or downloaded until the daemon endpoint is restored.'}
                </p>
                <p className="text-[11px] text-slate-400 italic">
                  Security policy enforced: Stale links and previous cached configurations are strictly suppressed.
                </p>
              </div>
            </div>
          )}

          {/* Node Summary Card */}
          {nodeConfig && nodeConfig.available && (
            <div className="glass-panel p-5 rounded-xl border border-slate-800 space-y-3 font-mono text-xs">
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 border-b border-slate-800 pb-3">
                <div className="space-y-1">
                  <div className="text-white font-bold text-sm tracking-wide flex items-center gap-2">
                    <span className="text-cyan-400">{selectedHost?.name}</span>
                    <span className="text-slate-500">/</span>
                    <span className="text-emerald-400">{nodeConfig.node_id}</span>
                  </div>
                  <div className="text-slate-400 text-[11px] flex items-center gap-3">
                    <span>IP: {nodeConfig.node_ip || selectedHost?.address}</span>
                    <span>•</span>
                    <span>Country: {nodeConfig.country || selectedHost?.region || 'GLOBAL'}</span>
                    {nodeConfig.endpoint_address && (
                      <>
                        <span>•</span>
                        <span>Endpoint: {nodeConfig.endpoint_address}:{nodeConfig.endpoint_port || 443}</span>
                      </>
                    )}
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  <span className="px-2.5 py-1 rounded bg-cyan-950/70 border border-cyan-800/80 text-cyan-300 font-bold text-[11px]">
                    {nodeConfig.protocol ? nodeConfig.protocol.toUpperCase() : 'VLESS'} REALITY / {nodeConfig.flow || 'VISION'}
                  </span>
                </div>
              </div>

              {/* Dynamic Profiles Section */}
              <div className="space-y-4 pt-2">
                <div className="flex items-center justify-between text-slate-400 text-xs font-bold uppercase tracking-wider">
                  <span>CANONICAL CLIENT PROFILES ({nodeConfig.profiles.length})</span>
                  <span className="text-[10px] text-slate-500 font-normal">Source: super-proxy daemon canonical export</span>
                </div>

                <div className="grid grid-cols-1 gap-4">
                  {nodeConfig.profiles.map((profile) => {
                    const isCopied = copiedKey === profile.id;
                    const canQR = profile.format === 'uri' || profile.can_qr;

                    return (
                      <div
                        key={profile.id}
                        className="bg-slate-900/90 rounded-xl border border-slate-800 p-4 space-y-3 hover:border-slate-700 transition"
                      >
                        <div className="flex items-center justify-between gap-3">
                          <div className="flex items-center gap-2">
                            <span className="text-sm font-bold text-white">{profile.name}</span>
                            <span className="px-2 py-0.5 rounded text-[10px] font-mono uppercase bg-slate-800 text-slate-300 border border-slate-700">
                              {profile.format}
                            </span>
                            {profile.description && (
                              <span className="text-xs text-slate-400 hidden sm:inline">• {profile.description}</span>
                            )}
                          </div>

                          <div className="flex items-center gap-1.5">
                            {/* Copy button */}
                            <button
                              onClick={() => handleCopyProfile(profile)}
                              className={`px-2.5 py-1 rounded text-xs font-bold transition flex items-center gap-1 ${
                                isCopied
                                  ? 'bg-emerald-900 text-emerald-200 border border-emerald-600'
                                  : 'bg-slate-800 hover:bg-slate-700 text-slate-200 border border-slate-700'
                              }`}
                              title="Copy configuration to clipboard"
                            >
                              {isCopied ? <Check className="w-3.5 h-3.5" /> : <Copy className="w-3.5 h-3.5" />}
                              <span>{isCopied ? 'Copied' : 'Copy'}</span>
                            </button>

                            {/* QR Code button (Only for URI format) */}
                            {canQR && (
                              <button
                                onClick={() => handleOpenQR(profile)}
                                className="px-2.5 py-1 rounded text-xs font-bold bg-slate-800 hover:bg-slate-700 text-cyan-300 border border-slate-700 transition flex items-center gap-1"
                                title="Show QR Code"
                              >
                                <QrCode className="w-3.5 h-3.5" />
                                <span>QR</span>
                              </button>
                            )}

                            {/* Download button */}
                            <button
                              onClick={() => handleDownloadProfile(profile)}
                              className="px-2.5 py-1 rounded text-xs font-bold bg-slate-800 hover:bg-slate-700 text-slate-300 border border-slate-700 transition flex items-center gap-1"
                              title={`Download as ${profile.filename || 'file'}`}
                            >
                              <Download className="w-3.5 h-3.5" />
                              <span>Download</span>
                            </button>
                          </div>
                        </div>

                        {/* Raw Content Preview Box */}
                        <div className="relative">
                          <pre className="p-3 bg-noc-950/80 rounded-lg border border-slate-800 text-[11px] font-mono text-slate-300 overflow-x-auto max-h-32 select-all whitespace-pre-wrap break-all">
                            {profile.content}
                          </pre>
                        </div>
                      </div>
                    );
                  })}
                </div>
              </div>
            </div>
          )}
        </>
      ) : (
        /* All Hosts Overview Mode */
        <div className="space-y-4 font-mono text-xs">
          <div className="p-4 glass-panel rounded-xl border border-slate-800 flex items-center justify-between">
            <div>
              <span className="text-white font-bold">ALL HOSTS & EGRESS NODES REGISTRY</span>
              <p className="text-xs text-slate-400 mt-0.5">
                Multi-host tree hierarchy. Configurations load on demand to conserve network bandwidth.
              </p>
            </div>
            <button
              onClick={fetchAllHostsOverview}
              className="flex items-center gap-1.5 px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg text-xs"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              Refresh
            </button>
          </div>

          <div className="space-y-3">
            {allHostsData.map(({ host, nodes: hostNodes }) => {
              const isExpanded = expandedHosts[host.id] ?? true;

              return (
                <div key={host.id} className="glass-panel rounded-xl border border-slate-800 overflow-hidden">
                  <div
                    onClick={() => setExpandedHosts({ ...expandedHosts, [host.id]: !isExpanded })}
                    className="p-4 bg-slate-900/60 hover:bg-slate-900 cursor-pointer flex items-center justify-between transition"
                  >
                    <div className="flex items-center gap-3">
                      {isExpanded ? <ChevronDown className="w-4 h-4 text-cyan-400" /> : <ChevronRight className="w-4 h-4 text-slate-400" />}
                      <span className="text-sm font-bold text-white">{host.name}</span>
                      <span className="text-slate-400">({host.address})</span>
                      {getStatusBadge(host.status)}
                    </div>
                    <span className="text-slate-400 text-xs">{hostNodes.length} Nodes</span>
                  </div>

                  {isExpanded && (
                    <div className="p-4 pt-0 divide-y divide-slate-800/60">
                      {hostNodes.length === 0 ? (
                        <div className="py-3 text-slate-500 text-center">No egress nodes currently active on this host</div>
                      ) : (
                        hostNodes.map((n) => (
                          <div key={n.id} className="py-3 flex items-center justify-between gap-3">
                            <div className="flex items-center gap-3">
                              <span className="w-2 h-2 rounded-full bg-emerald-400" />
                              <span className="text-white font-bold">{n.id}</span>
                              <span className="text-slate-400">{n.ip}</span>
                              <span className="px-1.5 py-0.5 rounded text-[10px] bg-slate-800 text-slate-300">
                                {n.country}
                              </span>
                            </div>

                            <button
                              onClick={() => {
                                setSelectedHost(host);
                                setSelectedHostID(host.id);
                                setSelectedNodeId(n.id);
                                setViewMode('single');
                              }}
                              className="px-3 py-1 rounded bg-cyan-950/80 hover:bg-cyan-900 text-cyan-300 border border-cyan-800 text-xs font-bold transition flex items-center gap-1.5"
                            >
                              <span>View Clients</span>
                              <ChevronRight className="w-3.5 h-3.5" />
                            </button>
                          </div>
                        ))
                      )}
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Batch Export Modal */}
      {isExportModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center p-4 bg-black/80 backdrop-blur-sm animate-in fade-in duration-150">
          <div className="relative w-full max-w-2xl p-6 glass-panel rounded-xl border border-cyan-500/30 glow-cyan font-mono text-xs space-y-4">
            <div className="flex items-center justify-between border-b border-slate-800 pb-3">
              <div className="flex items-center gap-2">
                <Archive className="w-5 h-5 text-cyan-400" />
                <h2 className="text-sm font-bold text-white tracking-wide">BATCH EXPORT CLIENT CONFIGURATIONS (ZIP)</h2>
              </div>
              <button
                onClick={() => setIsExportModalOpen(false)}
                className="p-1 text-slate-400 hover:text-white rounded-lg transition"
              >
                <X className="w-5 h-5" />
              </button>
            </div>

            <p className="text-slate-400 text-xs">
              Select which host egress nodes to package into a single ZIP archive. File contents are exported directly from canonical agent APIs (VLESS, Clash Meta, sing-box, Xray, Subscription).
            </p>

            {/* Tree Selector */}
            <div className="max-h-72 overflow-y-auto space-y-3 p-3 bg-noc-950/60 rounded-xl border border-slate-800">
              {allHostsData.map(({ host, nodes: hostNodes }) => (
                <div key={host.id} className="space-y-1.5">
                  <div className="flex items-center justify-between text-slate-300 font-bold text-xs pb-1 border-b border-slate-800/80">
                    <div className="flex items-center gap-2">
                      <Server className="w-3.5 h-3.5 text-cyan-400" />
                      <span>{host.name}</span>
                    </div>
                    <button
                      onClick={() => {
                        const next = { ...selectedForExport };
                        const allSelected = hostNodes.every((n) => next[`${host.id}:::${n.id}`]);
                        for (const n of hostNodes) {
                          next[`${host.id}:::${n.id}`] = !allSelected;
                        }
                        setSelectedForExport(next);
                      }}
                      className="text-[10px] text-cyan-400 hover:underline"
                    >
                      Toggle All
                    </button>
                  </div>

                  <div className="pl-4 space-y-1">
                    {hostNodes.map((n) => {
                      const key = `${host.id}:::${n.id}`;
                      const checked = !!selectedForExport[key];

                      return (
                        <div
                          key={n.id}
                          onClick={() => setSelectedForExport({ ...selectedForExport, [key]: !checked })}
                          className="flex items-center gap-2.5 py-1 text-slate-300 hover:text-white cursor-pointer"
                        >
                          {checked ? (
                            <CheckSquare className="w-4 h-4 text-cyan-400 shrink-0" />
                          ) : (
                            <Square className="w-4 h-4 text-slate-600 shrink-0" />
                          )}
                          <span>{n.id}</span>
                          <span className="text-slate-500">({n.ip} - {n.country})</span>
                        </div>
                      );
                    })}
                  </div>
                </div>
              ))}
            </div>

            <div className="flex items-center justify-between pt-2 border-t border-slate-800">
              <span className="text-slate-400">
                Selected: {Object.values(selectedForExport).filter(Boolean).length} nodes
              </span>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => setIsExportModalOpen(false)}
                  className="px-3 py-1.5 bg-slate-800 hover:bg-slate-700 text-slate-300 rounded-lg"
                >
                  Cancel
                </button>
                <button
                  onClick={handleBatchExport}
                  disabled={exportingZip}
                  className="px-4 py-1.5 bg-gradient-to-r from-cyan-600 to-blue-600 hover:from-cyan-500 hover:to-blue-500 text-white font-bold rounded-lg transition shadow flex items-center gap-1.5"
                >
                  <Download className="w-3.5 h-3.5" />
                  <span>{exportingZip ? 'Generating ZIP...' : 'Export Selected (ZIP)'}</span>
                </button>
              </div>
            </div>
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
