import React, { useState, useEffect } from 'react';
import { UserSession, DaemonStatus, SystemStats, Host } from './types';
import { api, getSelectedHostID, setSelectedHostID } from './api/client';
import { Navbar } from './components/Navbar';
import { Sidebar, TabId } from './components/Sidebar';
import { Login } from './pages/Login';
import { ChangePassword } from './pages/ChangePassword';
import { Dashboard } from './pages/Dashboard';
import { Hosts } from './pages/Hosts';
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
  const [hosts, setHosts] = useState<Host[]>([]);
  const [selectedHostID, setSelectedHostIDState] = useState<string>(getSelectedHostID());
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

  const fetchHostsList = async () => {
    try {
      const data = await api.listHosts().catch(() => []);
      setHosts(data || []);
      if (data && data.length > 0) {
        const saved = getSelectedHostID();
        if (!saved || !data.some((h) => h.id === saved)) {
          const def = data.find((h) => h.is_default) || data[0];
          setSelectedHostID(def.id);
          setSelectedHostIDState(def.id);
        }
      }
    } catch (e) {
      console.error('Failed to load hosts', e);
    }
  };

  // Poll global status and host stats
  const pollTelemetry = async () => {
    if (!session?.authenticated || session.must_change_password) return;
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
    if (session?.authenticated && !session.must_change_password) {
      fetchHostsList();
      pollTelemetry();
      const timer = setInterval(pollTelemetry, 5000);
      return () => clearInterval(timer);
    }
  }, [session?.authenticated, session?.must_change_password, selectedHostID]);

  const handleSelectHost = (hostID: string) => {
    setSelectedHostID(hostID);
    setSelectedHostIDState(hostID);
    pollTelemetry();
  };

  const handleLogout = async () => {
    await api.logout();
    setSession(null);
  };

  if (loading) {
    return (
      <div className="min-h-screen bg-noc-950 flex flex-col items-center justify-center font-mono text-xs text-cyan-400 space-y-3">
        <div className="h-6 w-6 border-2 border-cyan-500 border-t-transparent rounded-full animate-spin" />
        <span>INITIALIZING MULTI-HOST NOC CONTROL PLANE...</span>
      </div>
    );
  }

  if (!session?.authenticated) {
    return <Login onLoginSuccess={checkAuth} />;
  }

  // Force first-time password change before granting access to control panel
  if (session.must_change_password) {
    return <ChangePassword onPasswordChanged={checkAuth} />;
  }

  const isAdmin = session.role === 'admin';

  return (
    <div className="min-h-screen flex flex-col bg-noc-950 text-slate-100">
      {/* Top Navbar */}
      <Navbar
        session={session}
        status={daemonStatus}
        system={systemStats}
        hosts={hosts}
        selectedHostID={selectedHostID}
        onSelectHost={handleSelectHost}
        onNavigateToHosts={() => setCurrentTab('hosts')}
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
                hostsCount={hosts.length}
                selectedHostID={selectedHostID}
                onNavigateToHosts={() => setCurrentTab('hosts')}
                onNavigateToSlots={() => setCurrentTab('slots')}
                onNavigateToNodes={() => setCurrentTab('nodes')}
              />
            )}
            {currentTab === 'hosts' && (
              <Hosts
                isAdmin={isAdmin}
                onHostSelected={(id) => {
                  handleSelectHost(id);
                  setCurrentTab('dashboard');
                }}
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
