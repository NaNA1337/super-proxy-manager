import React, { useState, useEffect } from 'react';
import { UserSession, DaemonStatus, SystemStats } from './types';
import { api } from './api/client';
import { Navbar } from './components/Navbar';
import { Sidebar, TabId } from './components/Sidebar';
import { Login } from './pages/Login';
import { Dashboard } from './pages/Dashboard';
import { Nodes } from './pages/Nodes';
import { Slots } from './pages/Slots';
import { Routing } from './pages/Routing';
import { Metrics } from './pages/Metrics';
import { Events } from './pages/Events';
import { ShareLinks } from './pages/ShareLinks';
import { Subscriptions } from './pages/Subscriptions';
import { Settings } from './pages/Settings';
import { Audit } from './pages/Audit';

export const App: React.FC = () => {
  const [session, setSession] = useState<UserSession | null>(null);
  const [loading, setLoading] = useState(true);
  const [currentTab, setCurrentTab] = useState<TabId>('dashboard');
  const [daemonStatus, setDaemonStatus] = useState<DaemonStatus | null>(null);
  const [systemStats, setSystemStats] = useState<SystemStats | null>(null);
  const [isRefreshing, setIsRefreshing] = useState(false);

  // Check existing session on boot
  const checkAuth = async () => {
    try {
      const sess = await api.getMe();
      if (sess.authenticated) {
        setSession(sess);
      } else {
        setSession(null);
      }
    } catch {
      setSession(null);
    } finally {
      setLoading(false);
    }
  };

  // Poll global status and host stats
  const pollTelemetry = async () => {
    if (!session?.authenticated) return;
    setIsRefreshing(true);
    try {
      const [st, sys] = await Promise.all([
        api.getStatus().catch(() => null),
        api.getSystem().catch(() => null),
      ]);
      if (st) setDaemonStatus(st);
      if (sys) setSystemStats(sys);
    } finally {
      setIsRefreshing(false);
    }
  };

  useEffect(() => {
    checkAuth();
  }, []);

  useEffect(() => {
    if (session?.authenticated) {
      pollTelemetry();
      const timer = setInterval(pollTelemetry, 5000);
      return () => clearInterval(timer);
    }
  }, [session?.authenticated]);

  const handleLogout = async () => {
    await api.logout();
    setSession(null);
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-noc-950 flex flex-col items-center justify-center font-mono text-xs text-cyan-400 space-y-3">
        <div className="h-6 w-6 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin" />
        <span>INITIALIZING NOC CONTROL PLANE...</span>
      </div>
    );
  }

  if (!session?.authenticated) {
    return <Login onLoginSuccess={checkAuth} />;
  }

  const isAdmin = session.role === 'admin';

  return (
    <div className="min-h-screen flex flex-col bg-noc-950 text-slate-100">
      {/* Top Navbar */}
      <Navbar
        session={session}
        status={daemonStatus}
        system={systemStats}
        onLogout={handleLogout}
        onRefresh={pollTelemetry}
        isRefreshing={isRefreshing}
      />

      {/* Main Layout Body */}
      <div className="flex-1 flex overflow-hidden">
        {/* Sidebar */}
        <Sidebar currentTab={currentTab} onTabChange={setCurrentTab} />

        {/* Dynamic Page Content */}
        <main className="flex-1 overflow-y-auto p-4 sm:p-6 lg:p-8 bg-noc-950/60">
          <div className="max-w-7xl mx-auto">
            {currentTab === 'dashboard' && (
              <Dashboard
                status={daemonStatus}
                system={systemStats}
                onNavigateToSlots={() => setCurrentTab('slots')}
                onNavigateToNodes={() => setCurrentTab('nodes')}
              />
            )}
            {currentTab === 'nodes' && <Nodes onSelectNodeForShareLink={() => setCurrentTab('share-links')} />}
            {currentTab === 'slots' && <Slots isAdmin={isAdmin} />}
            {currentTab === 'routing' && <Routing />}
            {currentTab === 'metrics' && <Metrics />}
            {currentTab === 'events' && <Events />}
            {currentTab === 'share-links' && <ShareLinks />}
            {currentTab === 'subscriptions' && <Subscriptions isAdmin={isAdmin} />}
            {currentTab === 'settings' && <Settings />}
            {currentTab === 'audit' && <Audit />}
          </div>
        </main>
      </div>
    </div>
  );
};
