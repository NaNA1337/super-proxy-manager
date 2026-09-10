import React, { useState } from 'react';
import { Shield, KeyRound, AlertCircle, CheckCircle2, Lock } from 'lucide-react';
import { api } from '../api/client';

interface ChangePasswordProps {
  onPasswordChanged: () => void;
}

export const ChangePassword: React.FC<ChangePasswordProps> = ({ onPasswordChanged }) => {
  const [currentPassword, setCurrentPassword] = useState('');
  const [newPassword, setNewPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    if (newPassword.length < 12) {
      setError('New password must be at least 12 characters long.');
      return;
    }

    if (newPassword !== confirmPassword) {
      setError('New password and confirmation do not match.');
      return;
    }

    if (newPassword === currentPassword) {
      setError('New password cannot be the same as the temporary password.');
      return;
    }

    setLoading(true);
    try {
      await api.changePassword(currentPassword, newPassword, confirmPassword);
      onPasswordChanged();
    } catch (err: any) {
      setError(err.safeMessage || err.message || 'Failed to update password');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="min-h-screen bg-noc-950 flex flex-col items-center justify-center p-4">
      {/* Background glow */}
      <div className="absolute inset-0 overflow-hidden pointer-events-none">
        <div className="absolute -top-40 -left-40 w-96 h-96 bg-amber-500/10 rounded-full blur-3xl" />
        <div className="absolute -bottom-40 -right-40 w-96 h-96 bg-cyan-500/10 rounded-full blur-3xl" />
      </div>

      <div className="relative w-full max-w-md">
        {/* Header Branding */}
        <div className="flex flex-col items-center mb-6 text-center">
          <div className="p-3 rounded-xl bg-amber-500/10 border border-amber-500/30 text-amber-400 mb-3 shadow-lg shadow-amber-500/10">
            <Shield className="w-8 h-8" />
          </div>
          <h1 className="text-xl font-bold tracking-wider text-white font-mono uppercase">
            SUPER-PROXY MANAGER
          </h1>
          <p className="text-xs text-amber-400/90 font-mono mt-1 flex items-center gap-1">
            <Lock className="w-3.5 h-3.5" />
            FIRST-TIME SETUP: PASSWORD CHANGE REQUIRED
          </p>
        </div>

        {/* Form Panel */}
        <div className="glass-panel p-6 rounded-2xl border border-amber-500/30 shadow-2xl backdrop-blur-xl">
          <div className="mb-5 p-3 rounded-lg bg-amber-950/40 border border-amber-800/50 text-xs text-amber-300 flex items-start gap-2.5">
            <AlertCircle className="w-4 h-4 text-amber-400 shrink-0 mt-0.5" />
            <span>
              You are logged in with the <strong>temporary bootstrap password</strong>. For security compliance, you must set a persistent administrator password (minimum 12 characters) to unlock the control panel.
            </span>
          </div>

          {error && (
            <div className="mb-4 p-3 rounded-lg bg-rose-950/60 border border-rose-800/80 text-rose-300 text-xs flex items-center gap-2 animate-shake">
              <AlertCircle className="w-4 h-4 shrink-0" />
              <span>{error}</span>
            </div>
          )}

          <form onSubmit={handleSubmit} className="space-y-4">
            <div>
              <label className="block text-xs font-mono uppercase tracking-wider text-slate-400 mb-1.5">
                Current Temporary Password
              </label>
              <div className="relative">
                <input
                  type="password"
                  required
                  value={currentPassword}
                  onChange={(e) => setCurrentPassword(e.target.value)}
                  placeholder="Paste the temporary password from terminal"
                  className="w-full px-3.5 py-2.5 bg-slate-900/90 border border-slate-700/80 rounded-lg text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-amber-500 focus:ring-1 focus:ring-amber-500 font-mono transition"
                />
              </div>
            </div>

            <div>
              <div className="flex items-center justify-between mb-1.5">
                <label className="block text-xs font-mono uppercase tracking-wider text-slate-400">
                  New Administrator Password
                </label>
                <span className={`text-[10px] font-mono ${newPassword.length >= 12 ? 'text-emerald-400' : 'text-slate-500'}`}>
                  {newPassword.length}/12 min chars
                </span>
              </div>
              <input
                type="password"
                required
                value={newPassword}
                onChange={(e) => setNewPassword(e.target.value)}
                placeholder="Minimum 12 characters"
                className="w-full px-3.5 py-2.5 bg-slate-900/90 border border-slate-700/80 rounded-lg text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 focus:ring-1 focus:ring-cyan-500 font-mono transition"
              />
            </div>

            <div>
              <label className="block text-xs font-mono uppercase tracking-wider text-slate-400 mb-1.5">
                Confirm New Password
              </label>
              <input
                type="password"
                required
                value={confirmPassword}
                onChange={(e) => setConfirmPassword(e.target.value)}
                placeholder="Re-enter new password"
                className="w-full px-3.5 py-2.5 bg-slate-900/90 border border-slate-700/80 rounded-lg text-sm text-slate-100 placeholder-slate-500 focus:outline-none focus:border-cyan-500 focus:ring-1 focus:ring-cyan-500 font-mono transition"
              />
            </div>

            <div className="pt-2">
              <button
                type="submit"
                disabled={loading}
                className="w-full py-2.5 px-4 rounded-lg bg-gradient-to-r from-amber-600 to-cyan-600 hover:from-amber-500 hover:to-cyan-500 text-white font-medium text-sm transition shadow-lg shadow-cyan-950 flex items-center justify-center gap-2 disabled:opacity-50 active:scale-[0.98]"
              >
                {loading ? (
                  <div className="h-4 w-4 border-2 border-white border-t-transparent rounded-full animate-spin" />
                ) : (
                  <>
                    <KeyRound className="w-4 h-4" />
                    <span>Set Secure Password & Unlock Panel</span>
                  </>
                )}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  );
};
