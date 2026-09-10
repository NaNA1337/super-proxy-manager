import React from 'react';

interface StatusBadgeProps {
  status: string;
  className?: string;
  size?: 'sm' | 'md' | 'lg';
}

export const StatusBadge: React.FC<StatusBadgeProps> = ({ status, className = '', size = 'md' }) => {
  const s = (status || 'UNKNOWN').toUpperCase();

  let colorClasses = 'bg-slate-800 text-slate-300 border-slate-700';
  let dotColor = 'bg-slate-400';
  let glow = '';

  switch (s) {
    case 'ACTIVE':
      colorClasses = 'bg-emerald-950/80 text-emerald-300 border-emerald-500/50';
      dotColor = 'bg-emerald-400';
      glow = 'shadow-[0_0_8px_rgba(16,185,129,0.5)]';
      break;
    case 'STANDBY':
      colorClasses = 'bg-cyan-950/80 text-cyan-300 border-cyan-500/50';
      dotColor = 'bg-cyan-400';
      glow = 'shadow-[0_0_8px_rgba(6,182,212,0.5)]';
      break;
    case 'DRAINING':
      colorClasses = 'bg-amber-950/80 text-amber-300 border-amber-500/50';
      dotColor = 'bg-amber-400 animate-pulse';
      glow = 'shadow-[0_0_8px_rgba(245,158,11,0.5)]';
      break;
    case 'CONNECTING':
    case 'PREPARING':
    case 'VERIFYING':
      colorClasses = 'bg-blue-950/80 text-blue-300 border-blue-500/50';
      dotColor = 'bg-blue-400 animate-ping';
      glow = 'shadow-[0_0_8px_rgba(59,130,246,0.5)]';
      break;
    case 'FAILED':
      colorClasses = 'bg-rose-950/80 text-rose-300 border-rose-500/50';
      dotColor = 'bg-rose-400';
      glow = 'shadow-[0_0_8px_rgba(244,63,94,0.5)]';
      break;
    case 'COOLDOWN':
      colorClasses = 'bg-purple-950/80 text-purple-300 border-purple-500/50';
      dotColor = 'bg-purple-400';
      break;
    case 'DEAD':
      colorClasses = 'bg-red-950/90 text-red-400 border-red-800';
      dotColor = 'bg-red-600';
      break;
    case 'QUALIFIED':
    case 'DISCOVERED':
      colorClasses = 'bg-teal-950/80 text-teal-300 border-teal-500/40';
      dotColor = 'bg-teal-400';
      break;
    default:
      colorClasses = 'bg-slate-900 text-slate-400 border-slate-800';
      dotColor = 'bg-slate-500';
  }

  const sizeClasses = {
    sm: 'px-2 py-0.5 text-xs',
    md: 'px-2.5 py-1 text-xs font-semibold',
    lg: 'px-3 py-1.5 text-sm font-semibold',
  }[size];

  return (
    <span
      className={`inline-flex items-center gap-1.5 rounded-md border font-mono tracking-wider uppercase ${sizeClasses} ${colorClasses} ${glow} ${className}`}
    >
      <span className={`h-1.5 w-1.5 rounded-full ${dotColor}`} />
      <span>{s}</span>
    </span>
  );
};
