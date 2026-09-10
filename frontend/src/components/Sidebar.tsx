import React from 'react';
import {
  LayoutDashboard,
  Server,
  Layers,
  GitFork,
  BarChart3,
  ScrollText,
  Share2,
  Rss,
  Settings,
  ClipboardList
} from 'lucide-react';

export type TabId =
  | 'dashboard'
  | 'nodes'
  | 'slots'
  | 'routing'
  | 'metrics'
  | 'events'
  | 'share-links'
  | 'subscriptions'
  | 'settings'
  | 'audit';

interface SidebarProps {
  currentTab: TabId;
  onTabChange: (tab: TabId) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({ currentTab, onTabChange }) => {
  const navItems: { id: TabId; label: string; icon: React.ReactNode; badge?: string }[] = [
    { id: 'dashboard', label: 'Dashboard', icon: <LayoutDashboard className="w-4 h-4" /> },
    { id: 'nodes', label: 'Nodes', icon: <Server className="w-4 h-4" /> },
    { id: 'slots', label: 'Slots Manager', icon: <Layers className="w-4 h-4" /> },
    { id: 'routing', label: 'Policy Routing', icon: <GitFork className="w-4 h-4" /> },
    { id: 'metrics', label: 'Prom Metrics', icon: <BarChart3 className="w-4 h-4" /> },
    { id: 'events', label: 'Events & Logs', icon: <ScrollText className="w-4 h-4" /> },
    { id: 'share-links', label: 'Share Links', icon: <Share2 className="w-4 h-4" />, badge: 'VLESS' },
    { id: 'subscriptions', label: 'Subscriptions', icon: <Rss className="w-4 h-4" /> },
    { id: 'settings', label: 'Settings', icon: <Settings className="w-4 h-4" /> },
    { id: 'audit', label: 'Audit Log', icon: <ClipboardList className="w-4 h-4" /> },
  ];

  return (
    <aside className="w-64 glass-panel border-r border-slate-800/80 p-4 flex flex-col justify-between shrink-0 hidden md:flex">
      <div className="space-y-1">
        <div className="px-3 py-2 text-[10px] font-mono tracking-widest text-slate-500 uppercase font-bold">
          OPERATIONAL CONTROL
        </div>
        {navItems.map((item) => {
          const isActive = currentTab === item.id;
          return (
            <button
              key={item.id}
              onClick={() => onTabChange(item.id)}
              className={`w-full flex items-center justify-between px-3.5 py-2.5 rounded-lg text-xs font-medium transition ${
                isActive
                  ? 'bg-gradient-to-r from-cyan-500/20 to-blue-500/10 border border-cyan-500/40 text-cyan-300 font-semibold glow-cyan'
                  : 'text-slate-400 hover:text-slate-200 hover:bg-slate-800/50'
              }`}
            >
              <div className="flex items-center gap-3">
                <span className={isActive ? 'text-cyan-400' : 'text-slate-500'}>{item.icon}</span>
                <span>{item.label}</span>
              </div>
              {item.badge && (
                <span className="text-[9px] font-mono font-bold px-1.5 py-0.5 rounded bg-cyan-950 text-cyan-400 border border-cyan-800/40">
                  {item.badge}
                </span>
              )}
            </button>
          );
        })}
      </div>

      {/* Footer Security Badge */}
      <div className="p-3 rounded-lg bg-slate-900/60 border border-slate-800/60 text-[11px] text-slate-400 space-y-1 font-mono">
        <div className="flex items-center justify-between">
          <span>EGRESS ISOLATION:</span>
          <span className="text-emerald-400 font-bold">ACTIVE</span>
        </div>
        <div className="flex items-center justify-between">
          <span>LEAK PROTECTION:</span>
          <span className="text-emerald-400 font-bold">ENABLED</span>
        </div>
      </div>
    </aside>
  );
};
