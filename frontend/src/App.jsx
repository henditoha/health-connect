import React, { useState, useEffect } from 'react';
import { ArrowLeft, Save, Plus, TerminalSquare, ChevronDown, ChevronRight, Menu, LayoutDashboard, GitMerge, KeyRound, Users as UsersIcon, LogOut, ArrowUpRight, Layers, ShieldCheck, HeartPulse, Network, Award, Copy, Check } from './icons';
import ChannelEditor from './ChannelEditor';
import faviconLogo from '../assets/favicon.ico';

const API_BASE = import.meta.env.DEV ? 'http://localhost:3001/api' : '/api';

function TargetBadges({ endpoints }) {
  const targets = endpoints?.filter(e => e.role === 'target') || [];
  if (targets.length === 0) {
    return <span className="tag" style={{ backgroundColor: '#f3f4f6', color: '#6b7280', border: '1px solid #e5e7eb' }}>None</span>;
  }
  return targets.map((t, idx) => {
    let cfg = {};
    if (typeof t.config === 'string') {
      try { cfg = JSON.parse(t.config); } catch { cfg = {}; }
    } else if (typeof t.config === 'object' && t.config !== null) {
      cfg = t.config;
    }
    const name = cfg.name || `Target ${idx + 1}`;
    const hasFilter = !!cfg.filter_script?.trim();
    return (
      <span
        key={t.id || `${t.protocol}_${name}`}
        className="tag"
        style={{
          backgroundColor: '#eff6ff',
          color: '#1d4ed8',
          border: '1px solid #bfdbfe',
          display: 'inline-flex',
          alignItems: 'center',
          gap: '0.25rem'
        }}
        title={name}
      >
        {t.protocol?.toUpperCase() || 'HTTP'}
        {targets.length > 1 && (
          <span style={{ opacity: 0.8, fontSize: '0.7rem' }}>
            ({name.length > 12 ? name.slice(0, 10) + '..' : name})
          </span>
        )}
        {hasFilter && (
          <span title="Filter Active" style={{ color: '#d97706', fontSize: '0.65rem', fontWeight: 'bold' }}>⚡</span>
        )}
      </span>
    );
  });
}

function getLicenseBadge(licenseInfo) {
  const isValid = licenseInfo?.valid === true;
  const isTrial = licenseInfo?.is_trial === true;
  const isExpired = licenseInfo?.status_text === 'TRIAL_EXPIRED' || licenseInfo?.status_text === 'EXPIRED';

  if (isTrial && isExpired) {
    return { color: '#EF4444', bg: 'rgba(239, 68, 68, 0.12)', text: '✕ Trial Period Expired' };
  }
  if (isTrial) {
    return { color: '#D97706', bg: 'rgba(217, 119, 6, 0.12)', text: '⏱ Trial Mode (Evaluation)' };
  }
  if (isExpired) {
    return { color: '#EF4444', bg: 'rgba(239, 68, 68, 0.12)', text: '✕ License Expired' };
  }
  if (!isValid) {
    return { color: '#EF4444', bg: 'rgba(239, 68, 68, 0.12)', text: '✕ Unlicensed' };
  }
  return { color: '#10B981', bg: 'rgba(16, 185, 129, 0.12)', text: '✓ Official License Active' };
}

function getDaysLeftColor(daysLeft, isTrial) {
  if (daysLeft <= 3) return '#EF4444';
  if (isTrial) return '#D97706';
  return '#10B981';
}

function LicenseMachineCard({ machineCode, copiedCode, onCopy }) {
  return (
    <div style={{
      backgroundColor: 'var(--bg-card)',
      border: '1px solid var(--border)',
      borderRadius: '12px',
      padding: '1.5rem',
      boxShadow: '0 2px 4px rgba(0,0,0,0.03)'
    }}>
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: '1rem', flexWrap: 'wrap', gap: '0.5rem' }}>
        <div>
          <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: 'var(--primary)', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
            <Award size={20} color="var(--primary)" /> Laptop / Server Machine ID
          </h3>
          <p style={{ margin: '0.25rem 0 0', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            Unique hardware ID locked to your physical device (Windows, macOS, or Linux).
          </p>
        </div>
        <span style={{
          fontSize: '0.75rem',
          fontWeight: 700,
          padding: '0.25rem 0.6rem',
          borderRadius: '999px',
          backgroundColor: 'var(--primary-light)',
          color: 'var(--primary)'
        }}>
          Hardware ID
        </span>
      </div>

      <div style={{
        display: 'flex',
        alignItems: 'center',
        gap: '0.75rem',
        backgroundColor: 'var(--bg-subtle)',
        padding: '0.85rem 1.2rem',
        borderRadius: '8px',
        border: '1px dashed var(--border)',
        flexWrap: 'wrap'
      }}>
        <code style={{
          fontFamily: 'Consolas, Monaco, monospace',
          fontSize: '1.2rem',
          fontWeight: 700,
          letterSpacing: '0.08em',
          color: 'var(--primary)',
          flex: 1
        }}>
          {machineCode || 'Reading machine ID...'}
        </code>
        <button
          className="btn btn-primary"
          onClick={onCopy}
          style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 1rem' }}
        >
          {copiedCode ? <Check size={16} /> : <Copy size={16} />}
          {copiedCode ? 'Copied!' : 'Copy Machine ID'}
        </button>
      </div>
      <div style={{ marginTop: '0.75rem', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
        💡 Copy the <b>Machine ID</b> above and use it in the license generator to issue an official license key for your Hospital / Client.
      </div>
    </div>
  );
}

function LicenseStatusCard({ licenseInfo, totalRouteCount, activeRouteCount }) {
  const isValid = licenseInfo?.valid === true;
  const isTrial = licenseInfo?.is_trial === true;
  const isExpired = licenseInfo?.status_text === 'TRIAL_EXPIRED' || licenseInfo?.status_text === 'EXPIRED';
  const maxRoutes = licenseInfo?.max_routes || 10;
  const isExceeded = maxRoutes > 0 && totalRouteCount > maxRoutes;
  const badge = getLicenseBadge(licenseInfo);
  const daysLeftColor = getDaysLeftColor(licenseInfo?.days_left ?? 0, isTrial);

  const formattedExpiry = licenseInfo?.expires_at
    ? new Date(licenseInfo.expires_at).toLocaleDateString('en-US', { year: 'numeric', month: 'long', day: 'numeric' })
    : '-';

  const daysRemainingText = licenseInfo?.days_left === 0 ? 'Expires today' : `${licenseInfo?.days_left} days remaining`;
  const licenseTypeText = isTrial ? 'Trial (Evaluation Mode)' : (licenseInfo?.type || 'Production');
  const routeQuotaText = `${totalRouteCount} / ${maxRoutes > 0 ? maxRoutes : 'Unlimited'} Channels`;

  const customerDisplay = licenseInfo?.customer === 'Evaluasi Trial / Uji Coba'
    ? 'Trial Evaluation'
    : (licenseInfo?.customer || '-');

  const licenseMessageDisplay = licenseInfo?.message
    ? (licenseInfo.message
        .replace('Masa evaluasi Trial aktif sampai', 'Trial evaluation active until')
        .replace('Tersisa', '')
        .replace('hari', 'days remaining')
        .replace('Masa evaluasi Trial telah berakhir pada', 'Trial evaluation expired on')
        .replace('Silakan masukkan License Key untuk aktivasi penuh.', 'Please enter License Key for full activation.'))
    : 'Checking license status...';

  return (
    <div style={{
      backgroundColor: 'var(--bg-card)',
      border: '1px solid var(--border)',
      borderRadius: '12px',
      padding: '1.5rem',
      boxShadow: '0 2px 4px rgba(0,0,0,0.03)'
    }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '12px', flexWrap: 'wrap', gap: '0.5rem' }}>
        <div>
          <h3 style={{ margin: 0, fontSize: '1.1rem', fontWeight: 700, color: 'var(--primary)' }}>
            License Status & Validity
          </h3>
          <p style={{ margin: '0.2rem 0 0', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
            {licenseMessageDisplay}
          </p>
        </div>
        <span style={{
          padding: '0.4rem 0.9rem',
          borderRadius: '999px',
          fontSize: '0.85rem',
          fontWeight: 700,
          backgroundColor: badge.bg,
          color: badge.color,
          display: 'inline-flex',
          alignItems: 'center',
          gap: '0.4rem'
        }}>
          {badge.text}
        </span>
      </div>

      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(210px, 1fr))', gap: '1rem' }}>
        <div style={{ padding: '0.85rem', backgroundColor: 'var(--bg-subtle)', borderRadius: '8px' }}>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>License Type</div>
          <div style={{ fontWeight: 700, fontSize: '0.95rem', color: isTrial ? '#D97706' : 'var(--text-main)' }}>
            {licenseTypeText}
          </div>
        </div>

        <div style={{ padding: '0.85rem', backgroundColor: 'var(--bg-subtle)', borderRadius: '8px' }}>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>Organization / Client</div>
          <div style={{ fontWeight: 700, fontSize: '0.95rem', color: 'var(--text-main)' }}>
            {customerDisplay}
          </div>
        </div>

        <div style={{ padding: '0.85rem', backgroundColor: 'var(--bg-subtle)', borderRadius: '8px' }}>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>
            {isTrial ? 'Trial Expiration Date' : 'Expiration Date'}
          </div>
          <div style={{ fontWeight: 700, fontSize: '0.95rem', color: isExpired ? '#EF4444' : 'var(--text-main)' }}>
            {formattedExpiry}
          </div>
          {isValid && licenseInfo?.days_left !== undefined && (
            <div style={{
              fontSize: '0.75rem',
              color: daysLeftColor,
              marginTop: '0.2rem',
              fontWeight: 600
            }}>
              {daysRemainingText}
            </div>
          )}
        </div>

        <div style={{ padding: '0.85rem', backgroundColor: 'var(--bg-subtle)', borderRadius: '8px' }}>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginBottom: '0.25rem' }}>Channel Quota (Total)</div>
          <div style={{ fontWeight: 700, fontSize: '0.95rem', color: isExceeded ? '#EF4444' : 'var(--text-main)' }}>
            {routeQuotaText}
          </div>
          <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '0.2rem' }}>
            {activeRouteCount} Enabled | {totalRouteCount - activeRouteCount} Disabled
          </div>
          {isExceeded && (
            <div style={{ fontSize: '0.75rem', color: '#EF4444', marginTop: '0.2rem', fontWeight: 600 }}>
              License quota exceeded!
            </div>
          )}
        </div>
      </div>

      {isTrial && (
        <div style={{
          marginTop: '1rem',
          padding: '0.65rem 0.9rem',
          borderRadius: '6px',
          backgroundColor: 'rgba(217, 119, 6, 0.08)',
          border: '1px solid rgba(217, 119, 6, 0.2)',
          fontSize: '0.8rem',
          color: '#B45309'
        }}>
          ℹ️ <b>Trial Evaluation Terms:</b> Maximum limit of <b>10 channels</b> (counting both Enabled and Disabled). Created channels <b>cannot be deleted</b> during the trial period.
        </div>
      )}

      <div style={{ marginTop: '1rem', paddingTop: '1rem', borderTop: '1px solid var(--border-light)', fontSize: '0.8rem', color: 'var(--text-muted)' }}>
        📁 <b>File Location on Machine:</b>{' '}
        <code style={{ fontFamily: 'monospace', color: 'var(--primary)' }}>
          {licenseInfo?.file_path || 'license.key'}
        </code>
      </div>
    </div>
  );
}

function LicenseActivationCard({ licenseKeyInput, setLicenseKeyInput, licenseMsg, setLicenseMsg, onActivate, machineCode }) {
  const isSuccess = licenseMsg?.type === 'success';
  const alertBg = isSuccess ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)';
  const alertColor = isSuccess ? '#047857' : '#DC2626';
  const alertBorder = isSuccess ? '#A7F3D0' : '#FECACA';

  return (
    <div style={{
      backgroundColor: 'var(--bg-card)',
      border: '1px solid var(--border)',
      borderRadius: '12px',
      padding: '1.5rem',
      boxShadow: '0 2px 4px rgba(0,0,0,0.03)'
    }}>
      <h3 style={{ margin: '0 0 0.5rem', fontSize: '1.1rem', fontWeight: 700, color: 'var(--primary)' }}>
        Enter Generated License Key
      </h3>
      <p style={{ margin: '0 0 1rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
        Paste the encrypted License Key generated from the license tool for this Machine ID.
      </p>

      {licenseMsg?.text && (
        <div style={{
          padding: '0.75rem 1rem',
          borderRadius: '6px',
          marginBottom: '1rem',
          fontSize: '0.875rem',
          backgroundColor: alertBg,
          color: alertColor,
          border: `1px solid ${alertBorder}`
        }}>
          {licenseMsg.text}
        </div>
      )}

      <form onSubmit={onActivate}>
        <textarea
          className="form-control"
          rows={4}
          placeholder="Paste License Key text here (e.g. eyJjdXN0b21lciI6...)"
          value={licenseKeyInput}
          onChange={e => setLicenseKeyInput(e.target.value)}
          style={{
            width: '100%',
            fontFamily: 'Consolas, Monaco, monospace',
            fontSize: '0.85rem',
            padding: '0.75rem',
            borderRadius: '8px',
            border: '1px solid var(--border)',
            marginBottom: '1rem',
            resize: 'vertical'
          }}
        />
        <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem' }}>
          <button
            type="button"
            className="btn"
            onClick={() => { setLicenseKeyInput(''); setLicenseMsg({ text: '', type: '' }); }}
            disabled={!licenseKeyInput && !licenseMsg?.text}
          >
            Clear
          </button>
          <button type="submit" className="btn btn-primary" style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem' }}>
            <ShieldCheck size={16} /> Activate License Key
          </button>
        </div>
      </form>

      <div style={{
        marginTop: '1.25rem',
        padding: '1rem',
        backgroundColor: 'var(--bg-subtle)',
        borderRadius: '8px',
        border: '1px solid var(--border-light)',
        fontSize: '0.82rem',
        color: 'var(--text-muted)'
      }}>
        <div style={{ fontWeight: 700, color: 'var(--primary)', marginBottom: '0.35rem' }}>
          🛠 How to Issue License Key via CLI Generator (Admin):
        </div>
        <code>
          go run ./tools/license-gen -machine &quot;{machineCode || 'MACHINE-ID'}&quot; -customer &quot;RS Hermina&quot; -days 365 -max-routes 10
        </code>
      </div>
    </div>
  );
}

function App() {
  const [routes, setRoutes] = useState([]);
  const [logs, setLogs] = useState([]);
  const [activeTab, setActiveTab] = useState('integrations');
  const [selectedChannel, setSelectedChannel] = useState('');
  const [selectedLog, setSelectedLog] = useState(null);
  const [logViewTab, setLogViewTab] = useState('inbound');
  const [editingRoute, setEditingRoute] = useState(null);
  const [selectedRouteId, setSelectedRouteId] = useState(null);
  const [collapsedGroups, setCollapsedGroups] = useState({});
  const [groups, setGroups] = useState([]);
  const [variables, setVariables] = useState([]);
  const [logSearch, setLogSearch] = useState('');
  const [logStartDate, setLogStartDate] = useState('');
  const [logEndDate, setLogEndDate] = useState('');
  const [logPage, setLogPage] = useState(1);
  const [logTotalPages, setLogTotalPages] = useState(1);
  const [logTableHeight, setLogTableHeight] = useState(350);
  const [testStatus, setTestStatus] = useState({ source: null, target: null });

  const [token, setToken] = useState(localStorage.getItem('token') || '');
  const [users, setUsers] = useState([]);
  const [loginUser, setLoginUser] = useState('');
  const [loginPass, setLoginPass] = useState('');
  const [loginError, setLoginError] = useState('');

  const [loading, setLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [stats, setStats] = useState({});

  const [licenseInfo, setLicenseInfo] = useState(null);
  const [licenseKeyInput, setLicenseKeyInput] = useState('');
  const [licenseMsg, setLicenseMsg] = useState({ text: '', type: '' });
  const [copiedCode, setCopiedCode] = useState(false);
  const [initialRouteSnapshot, setInitialRouteSnapshot] = useState(null);

  const cleanRouteForComparison = (r) => {
    if (!r) return '';
    const { activeTargetIdx, channelTab, ...persistedData } = r;
    return JSON.stringify(persistedData);
  };

  const isRouteDirty = () => {
    if (!editingRoute || !initialRouteSnapshot) return false;
    return cleanRouteForComparison(editingRoute) !== initialRouteSnapshot;
  };

  const navigateToTab = (tab) => {
    if (editingRoute) {
      if (isRouteDirty()) {
        const tabLabels = {
          dashboard: 'Dashboard',
          integrations: 'Channels',
          environments: 'Environments',
          license: 'License & Machine',
          users: 'Users',
          logs: 'Message Logs'
        };
        const targetLabel = tabLabels[tab] || tab;
        const confirmLeave = window.confirm(
          `You have unsaved changes in channel "${editingRoute.name || 'Untitled'}". Discard changes and go to ${targetLabel}?`
        );
        if (!confirmLeave) return;
      }
      setEditingRoute(null);
      setInitialRouteSnapshot(null);
    }
    setActiveTab(tab);
    setIsSidebarOpen(false);
    if (tab === 'environments') void fetchVariables();
    if (tab === 'license') void fetchLicense();
    if (tab === 'users') void fetchUsers();
  };

  const handleCloseEditor = () => {
    if (editingRoute && isRouteDirty()) {
      const confirmLeave = window.confirm(
        `You have unsaved changes in channel "${editingRoute.name || 'Untitled'}". Discard changes and return to Channels?`
      );
      if (!confirmLeave) return;
    }
    setEditingRoute(null);
    setInitialRouteSnapshot(null);
  };

  useEffect(() => {
    const handleBeforeUnload = (e) => {
      if (editingRoute && isRouteDirty()) {
        e.preventDefault();
        e.returnValue = '';
      }
    };
    window.addEventListener('beforeunload', handleBeforeUnload);
    return () => window.removeEventListener('beforeunload', handleBeforeUnload);
  }, [editingRoute, initialRouteSnapshot]);

  useEffect(() => {
    if (token) {
      void fetchGroups();
      void fetchRoutes();
      void fetchVariables();
      void fetchLogs(1);
      void fetchStats();
      void fetchLicense();
      const timer = setInterval(() => { void fetchStats(); }, 5000);
      return () => clearInterval(timer);
    }
  }, [token]);

  const fetchStats = async () => {
    try {
      const res = await fetch(`${API_BASE}/stats`);
      const data = await res.json();
      if (data && typeof data === 'object' && !Array.isArray(data)) setStats(data);
    } catch (e) {
      console.error("Failed to fetch stats", e);
    }
  };

  const fetchGroups = async () => {
    try {
      const res = await fetch(`${API_BASE}/groups`);
      const data = await res.json();
      if (Array.isArray(data)) {
        setGroups(data);
      } else {
        setGroups([]);
      }
    } catch (e) {
      console.error("Failed to fetch groups", e);
      setGroups([]);
    }
  };

  const fetchVariables = async () => {
    try {
      const res = await fetch(`${API_BASE}/variables`);
      const data = await res.json();
      if (Array.isArray(data)) {
        setVariables(data);
      } else {
        setVariables([]);
      }
    } catch (e) {
      console.error("Failed to fetch variables", e);
      setVariables([]);
    }
  };

  const fetchRoutes = async () => {
    try {
      const res = await fetch(`${API_BASE}/routes`);
      const data = await res.json();
      if (Array.isArray(data)) {
        setRoutes(data);
      } else {
        console.error("Failed to load routes:", data);
        setRoutes([]);
      }
    } catch (e) {
      console.error("Failed to fetch routes", e);
      setRoutes([]);
    } finally {
      setLoading(false);
    }
  };

  const fetchLogs = async (page = 1) => {
    try {
      const params = new URLSearchParams({
        page: page,
        limit: 100
      });
      if (logSearch) params.append('search', logSearch);
      if (logStartDate) params.append('startDate', logStartDate);
      if (logEndDate) params.append('endDate', logEndDate);
      if (selectedChannel) params.append('channel', selectedChannel);

      const res = await fetch(`${API_BASE}/logs?${params.toString()}`);
      const data = await res.json();
      if (data.logs) {
        setLogs(data.logs);
        setLogPage(data.page);
        setLogTotalPages(data.totalPages);
      } else if (Array.isArray(data)) {
        setLogs(data); // Fallback for old API
      } else {
        console.error("Failed to load logs:", data);
        setLogs([]);
      }
    } catch (e) {
      console.error("Failed to fetch logs", e);
      setLogs([]);
    }
  };

  const createRoute = async () => {
    const name = prompt("Enter channel name:");
    if (!name) return;

    try {
      const res = await fetch(`${API_BASE}/routes`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, description: 'New healthcare channel', group_name: '' })
      });
      const data = await res.json();
      if (!res.ok) {
        alert(data.error || "Failed to create channel.");
        return;
      }
      void fetchRoutes();
    } catch (e) {
      console.error(e);
      alert("Failed to connect to server");
    }
  };

  const createGroup = async () => {
    const name = prompt("Enter new Group name (e.g. Satu Sehat):");
    if (!name) return;
    try {
      await fetch(`${API_BASE}/groups`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ name, description: '' })
      });
      void fetchGroups();
    } catch (e) {
      console.error(e);
    }
  };

  const toggleRoute = async (id) => {
    try {
      const res = await fetch(`${API_BASE}/routes/${id}/toggle`, { method: 'PUT' });
      if (!res.ok) {
        const data = await res.json();
        alert(data.error || "Failed to update channel status.");
        return;
      }
      void fetchRoutes();
    } catch (e) {
      console.error(e);
    }
  };

  const cloneRoute = async (id) => {
    try {
      const res = await fetch(`${API_BASE}/routes/${id}/clone`, { method: 'POST' });
      const data = await res.json();
      if (!res.ok) {
        alert(data.error || "Failed to clone channel.");
        return;
      }
      void fetchRoutes();
    } catch (e) {
      console.error(e);
      alert("Failed to connect to server");
    }
  };

  const deleteRoute = async (id) => {
    if (licenseInfo?.is_trial) {
      alert("Channels cannot be deleted during Trial evaluation mode.");
      return;
    }
    const route = routes.find(r => r.id === id);
    if (!confirm(`Are you sure you want to delete channel "${route?.name || id}"?`)) {
      return;
    }
    try {
      const res = await fetch(`${API_BASE}/routes/${id}`, { method: 'DELETE' });
      const data = await res.json();
      if (!res.ok) {
        alert(data.error || "Failed to delete channel.");
        return;
      }
      setSelectedRouteId(null);
      void fetchRoutes();
    } catch (e) {
      console.error(e);
      alert("Failed to connect to server");
    }
  };

  const saveRoute = async () => {
    try {
      const combinedVars = {
        ...editingRoute.source?.variables
      };
      editingRoute.targets?.forEach(t => {
        if (t.config?.variables) {
          Object.assign(combinedVars, t.config.variables);
        }
      });

      const targetsPayload = (editingRoute.targets || []).map(t => ({
        id: t.id,
        protocol: t.protocol,
        format: t.format,
        config: {
          ...t.config,
          filter_script: t.config.filter_script || '',
          transformer_script: t.config.transformer_script || ''
        }
      }));

      const primaryTarget = targetsPayload[0] || { protocol: 'http', format: 'json', config: {} };

      const payload = {
        name: editingRoute.name,
        description: editingRoute.description,
        group_name: editingRoute.group_name || '',
        source: {
          protocol: editingRoute.source.protocol,
          format: editingRoute.source.format,
          config: {
            ...editingRoute.source.config,
            variables: editingRoute.source.variables || {},
            filter_script: editingRoute.source.config.filter_script || '',
            transformer_script: editingRoute.source.config.transformer_script || ''
          }
        },
        target: primaryTarget,
        targets: targetsPayload,
        transformer: {
          source_script: editingRoute.source.config.transformer_script || editingRoute.transformer?.source_script || '',
          script: primaryTarget.config.transformer_script || editingRoute.transformer?.script || '',
          inbound_template: editingRoute.source.config.inbound_template || editingRoute.transformer?.inbound_template || '',
          outbound_template: primaryTarget.config.outbound_template || editingRoute.transformer?.outbound_template || ''
        },
        variables: combinedVars
      };

      const res = await fetch(`${API_BASE}/routes/${editingRoute.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });

      if (!res.ok) {
        const errText = await res.text();
        throw new Error(errText);
      }

      setEditingRoute(null);
      setInitialRouteSnapshot(null);
      void fetchRoutes();
    } catch (e) {
      console.error(e);
      alert("Failed to save route: " + e.message);
    }
  };

  const testConnection = async (role, targetIdx = 0) => {
    const statusKey = role === 'source' ? 'source' : `target_${targetIdx}`;
    setTestStatus(prev => ({ ...prev, [statusKey]: { state: 'loading', message: '' } }));
    
    const endpoint = role === 'source' 
      ? editingRoute.source 
      : (editingRoute.targets?.[targetIdx] || editingRoute.targets?.[0]);
      
    // Collect all variables so references work across the board
    const allVars = {
      ...editingRoute.source?.variables
    };
    editingRoute.targets?.forEach(t => {
      if (t.config?.variables) Object.assign(allVars, t.config.variables);
    });

    try {
      const res = await fetch(`${API_BASE}/test-connection`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          role,
          protocol: endpoint.protocol,
          config: endpoint.config,
          local_variables: JSON.stringify(allVars)
        })
      });
      const data = await res.json();
      if (res.ok && data.success) {
        setTestStatus(prev => ({ ...prev, [statusKey]: { state: 'success', message: data.message } }));
      } else {
        setTestStatus(prev => ({ ...prev, [statusKey]: { state: 'error', message: data.error || 'Unknown error' } }));
      }
    } catch (e) {
      setTestStatus(prev => ({ ...prev, [statusKey]: { state: 'error', message: e.message } }));
    }
  };

  const openEdit = (route) => {
    setTestStatus({});
    const sourceEp = route.endpoints?.find(e => e.role === 'source') || {
      role: 'source',
      protocol: 'http',
      format: 'json',
      config: '{}'
    };
    const targetEps = route.endpoints?.filter(e => e.role === 'target') || [];
    
    const parsedSourceCfg = typeof sourceEp.config === 'string' ? JSON.parse(sourceEp.config || '{}') : (sourceEp.config || {});
    
    // Normalize target destinations: at least 1 destination
    let destinations = targetEps.map((ep, idx) => {
      const cfg = typeof ep.config === 'string' ? JSON.parse(ep.config || '{}') : (ep.config || {});
      return {
        id: ep.id || `dest_${Date.now()}_${idx}`,
        protocol: ep.protocol || 'http',
        format: ep.format || 'json',
        config: {
          name: cfg.name || `Destination ${idx + 1}`,
          enabled: cfg.enabled !== false,
          filter_script: cfg.filter_script || '',
          transformer_script: cfg.transformer_script || (idx === 0 ? (route.transformer?.script || '') : ''),
          outbound_template: cfg.outbound_template || (idx === 0 ? (route.transformer?.outbound_template || '') : ''),
          ...cfg
        }
      };
    });

    if (destinations.length === 0) {
      destinations = [{
        id: `dest_${Date.now()}`,
        protocol: 'http',
        format: 'json',
        config: {
          name: 'Primary Destination',
          enabled: true,
          host: '',
          port: '',
          path: '',
          authType: 'none',
          filter_script: '',
          transformer_script: route.transformer?.script || '',
          outbound_template: route.transformer?.outbound_template || ''
        }
      }];
    }

    // Source variables
    const sourceVars = { ...parsedSourceCfg.variables };
    Object.entries(route.variables || {}).forEach(([k, v]) => {
      if (!sourceVars[k]) sourceVars[k] = v;
    });

    const preparedRoute = {
      ...route,
      source: {
        ...sourceEp,
        protocol: sourceEp.protocol || 'http',
        format: sourceEp.format || 'json',
        config: {
          filter_script: parsedSourceCfg.filter_script || '',
          transformer_script: parsedSourceCfg.transformer_script || (route.transformer?.source_script || ''),
          inbound_template: parsedSourceCfg.inbound_template || (route.transformer?.inbound_template || ''),
          ...parsedSourceCfg
        },
        variables: sourceVars
      },
      targets: destinations,
      activeTargetIdx: 0,
      channelTab: 'source', // 'source' or 'destinations'
      transformer: route.transformer || { script: '', source_script: '', inbound_template: '', outbound_template: '' }
    };

    setEditingRoute(preparedRoute);
    setInitialRouteSnapshot(cleanRouteForComparison(preparedRoute));
  };

  const getLogContent = () => {
    if (!selectedLog) return '';
    if (logViewTab === 'inbound') return selectedLog.inbound_data || '';
    if (logViewTab === 'outbound') return selectedLog.outbound_data || '';
    return selectedLog.response_data || selectedLog.error_message || 'No response received.';
  };

  const renderIntegrations = () => {
    const groupedRoutes = routes.reduce((acc, route) => {
      const group = route.group_name || 'Ungrouped';
      if (!acc[group]) acc[group] = [];
      acc[group].push(route);
      return acc;
    }, {});

    return (
      <div className="table-wrapper">
        <table className="mirth-table">
          <thead>
            <tr>
              <th style={{ width: '130px' }}>Status</th>
              <th>Channel Name</th>
              <th>Description</th>
              <th style={{ width: '90px', textAlign: 'right' }}>Received</th>
              <th style={{ width: '90px', textAlign: 'right' }}>Filtered</th>
              <th style={{ width: '80px', textAlign: 'right' }}>Queue</th>
              <th style={{ width: '80px', textAlign: 'right' }}>Sent</th>
              <th style={{ width: '80px', textAlign: 'right' }}>Error</th>
              <th style={{ width: '120px' }}>Connection</th>
              <th style={{ width: '110px', textAlign: 'right' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {Object.entries(groupedRoutes).map(([groupName, groupRoutes]) => {
              const isCollapsed = collapsedGroups[groupName];

              return (
                <React.Fragment key={groupName}>
                  <tr className="group-row">
                    <td colSpan="10">
                      <div className="group-row-content">
                        <button
                          className="group-toggle-btn"
                          onClick={() => setCollapsedGroups({ ...collapsedGroups, [groupName]: !isCollapsed })}
                        >
                          {isCollapsed ? <ChevronRight size={15} /> : <ChevronDown size={15} />}
                        </button>
                        <span>{groupName}</span>
                        <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)', fontWeight: 500 }}>
                          ({groupRoutes.length} {groupRoutes.length === 1 ? 'channel' : 'channels'})
                        </span>
                      </div>
                    </td>
                  </tr>
                  {!isCollapsed && groupRoutes.map(route => {
                    const isSelected = selectedRouteId === route.id;
                    const rStats = stats[route.id] || {};
                    const isEnabled = route.active !== false;

                    return (
                      <tr
                        key={route.id}
                        onClick={() => setSelectedRouteId(route.id)}
                        className={isSelected ? 'selected' : ''}
                      >
                        <td style={{ paddingLeft: '1.75rem' }}>
                          <span className={`status-pill ${isEnabled ? 'active' : 'inactive'}`}>
                            <span className="status-dot"></span>
                            {isEnabled ? 'Running' : 'Disabled'}
                          </span>
                        </td>
                        <td>
                          <div style={{ display: 'flex', flexDirection: 'column' }}>
                            <span style={{ fontWeight: 700, color: 'var(--text-main)', fontSize: '0.88rem' }}>{route.name}</span>
                            <span style={{ fontSize: '0.72rem', color: 'var(--text-muted)' }}>ID: {route.id.slice(0, 8)}</span>
                          </div>
                        </td>
                        <td style={{ color: 'var(--text-secondary)' }}>{route.description || <span style={{ color: 'var(--text-muted)', fontStyle: 'italic' }}>No description</span>}</td>
                        <td style={{ textAlign: 'right' }}><span className="num-stat">{rStats.received ?? 0}</span></td>
                        <td style={{ textAlign: 'right' }}><span className="num-stat">{rStats.filtered ?? 0}</span></td>
                        <td style={{ textAlign: 'right' }}><span className="num-stat">{rStats.queue ?? 0}</span></td>
                        <td style={{ textAlign: 'right' }}><span className="num-stat" style={{ color: 'var(--success-dark)' }}>{rStats.sent ?? 0}</span></td>
                        <td style={{ textAlign: 'right' }}>
                          <span className={`num-stat ${(rStats.error ?? 0) > 0 ? 'error' : ''}`}>{rStats.error ?? 0}</span>
                        </td>
                        <td>
                          {(rStats.connections ?? 0) > 0 ? (
                            <span className="conn-badge connected">
                              <span style={{ width: 6, height: 6, borderRadius: '50%', background: 'var(--success)' }}></span>
                              {rStats.connections} active
                            </span>
                          ) : (
                            <span className="conn-badge idle">Idle</span>
                          )}
                        </td>
                        <td style={{ textAlign: 'right' }}>
                          <div style={{ display: 'flex', gap: '0.4rem', justifyContent: 'flex-end' }}>
                            <button
                              className="btn btn-secondary"
                              style={{ padding: '0.35rem 0.65rem', fontSize: '0.75rem' }}
                              onClick={(e) => {
                                e.stopPropagation();
                                setActiveTab('logs');
                                setSelectedChannel(route.name);
                                setSelectedLog(null);
                                void fetchLogs();
                              }}
                            >
                              <TerminalSquare size={13} /> Logs
                            </button>
                          </div>
                        </td>
                      </tr>
                    );
                  })}
                </React.Fragment>
              );
            })}
            {routes.length === 0 && (
              <tr>
                <td colSpan="10" style={{ textAlign: 'center', padding: '3.5rem 1rem', color: 'var(--text-muted)' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', gap: '0.5rem' }}>
                    <Layers size={32} color="#CBD5E1" />
                    <span style={{ fontWeight: 600 }}>No healthcare channels configured</span>
                    <span style={{ fontSize: '0.8rem' }}>Create your first integration route via the sidebar menu.</span>
                  </div>
                </td>
              </tr>
            )}
          </tbody>
        </table>
      </div>
    );
  };

  const renderLogsView = () => (
    <div className="logs-container" style={{ display: 'flex', flexDirection: 'column', height: 'calc(100vh - 120px)', gap: '1rem' }}>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', backgroundColor: 'var(--bg-card)', padding: '1rem', borderRadius: '8px', border: '1px solid var(--border)' }}>
        <div style={{ display: 'flex', gap: '1rem', alignItems: 'center', flexWrap: 'wrap' }}>
          <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
            <label htmlFor="channel-select" style={{ fontWeight: 600, color: 'var(--text-muted)' }}>Channel:</label>
            <select
              id="channel-select"
              className="form-control"
              style={{ width: '200px', backgroundColor: 'var(--bg)' }}
              value={selectedChannel}
              onChange={e => { setSelectedChannel(e.target.value); setSelectedLog(null); }}
            >
              <option value="">-- All Channels --</option>
              {routes.map(r => (
                <option key={r.id} value={r.name}>{r.name}</option>
              ))}
            </select>
          </div>

          <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
            <label htmlFor="log-search-input" style={{ fontWeight: 600, color: 'var(--text-muted)' }}>Search:</label>
            <input id="log-search-input" type="text" className="form-control" placeholder="Search content..." value={logSearch} onChange={e => setLogSearch(e.target.value)} style={{ width: '200px', backgroundColor: 'var(--bg)' }} />
          </div>

          <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
            <label htmlFor="log-start-input" style={{ fontWeight: 600, color: 'var(--text-muted)' }}>Start:</label>
            <input id="log-start-input" type="datetime-local" className="form-control" value={logStartDate} onChange={e => setLogStartDate(e.target.value)} style={{ backgroundColor: 'var(--bg)' }} />
          </div>

          <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
            <label htmlFor="log-end-input" style={{ fontWeight: 600, color: 'var(--text-muted)' }}>End:</label>
            <input id="log-end-input" type="datetime-local" className="form-control" value={logEndDate} onChange={e => setLogEndDate(e.target.value)} style={{ backgroundColor: 'var(--bg)' }} />
          </div>

          <button className="btn btn-primary" onClick={() => fetchLogs(1)}>Search Logs</button>
        </div>
      </div>

      <div style={{ height: `${logTableHeight}px`, minHeight: '150px', flex: 'none', backgroundColor: 'var(--bg-card)', border: '1px solid var(--border)', overflowY: 'auto', borderRadius: '8px' }}>
        <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
          <thead style={{ position: 'sticky', top: 0, zIndex: 10 }}>
            <tr>
              <th style={{ padding: '0.5rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>ID</th>
              <th style={{ padding: '0.5rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Source</th>
              <th style={{ padding: '0.5rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Target</th>
              <th style={{ padding: '0.5rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Status</th>
              <th style={{ padding: '0.5rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Date</th>
              <th style={{ padding: '0.5rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Error</th>
            </tr>
          </thead>
          <tbody>
            {logs.map((log) => (
              <tr
                key={log.id}
                style={{
                  borderBottom: '1px solid var(--border)',
                  cursor: 'pointer',
                  backgroundColor: selectedLog?.id === log.id ? 'rgba(79, 70, 229, 0.2)' : 'transparent',
                  transition: 'background-color 0.1s'
                }}
                onClick={() => setSelectedLog(log)}
              >
                <td style={{ padding: '0.75rem 1rem', fontSize: '0.85rem' }}>{log.id.substring(0, 8)}...</td>
                <td style={{ padding: '0.75rem 1rem', fontSize: '0.85rem' }}>{log.source_protocol}</td>
                <td style={{ padding: '0.75rem 1rem', fontSize: '0.85rem' }}>{log.target_protocol}</td>
                <td style={{ padding: '0.75rem 1rem' }}>
                  <span style={{
                    padding: '0.2rem 0.4rem', borderRadius: '4px', fontSize: '0.75rem', fontWeight: 'bold',
                    backgroundColor: log.status === 'SUCCESS' ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
                    color: log.status === 'SUCCESS' ? '#10B981' : '#EF4444'
                  }}>
                    {log.status}
                  </span>
                </td>
                <td style={{ padding: '0.75rem 1rem', fontSize: '0.85rem' }}>{new Date(log.created_at).toLocaleString()}</td>
                <td style={{ padding: '0.75rem 1rem', fontSize: '0.85rem', color: '#EF4444' }}>{log.error_message || '--'}</td>
              </tr>
            ))}
            {logs.length === 0 && (
              <tr><td colSpan="6" style={{ padding: '2rem', textAlign: 'center', color: 'var(--text-muted)' }}>No messages found.</td></tr>
            )}
          </tbody>
        </table>
      </div>

      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: '0 0.5rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
        <span>Showing up to 100 logs per page</span>
        <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
          <button className="btn" style={{ padding: '0.25rem 0.5rem', backgroundColor: 'var(--bg-card)', border: '1px solid var(--border)' }} disabled={logPage <= 1} onClick={() => fetchLogs(logPage - 1)}>Prev</button>
          <span>Page {logPage} of {logTotalPages}</span>
          <button className="btn" style={{ padding: '0.25rem 0.5rem', backgroundColor: 'var(--bg-card)', border: '1px solid var(--border)' }} disabled={logPage >= logTotalPages} onClick={() => fetchLogs(logPage + 1)}>Next</button>
        </div>
      </div>

      <button
        onMouseDown={(e) => {
          e.preventDefault();
          const handleMouseMove = (moveEvent) => setLogTableHeight(prev => Math.max(100, prev + moveEvent.movementY));
          const handleMouseUp = () => {
            document.removeEventListener('mousemove', handleMouseMove);
            document.removeEventListener('mouseup', handleMouseUp);
          };
          document.addEventListener('mousemove', handleMouseMove);
          document.addEventListener('mouseup', handleMouseUp);
        }}
        style={{
          width: '100%',
          border: 'none',
          padding: 0,
          height: '12px',
          cursor: 'row-resize',
          backgroundColor: 'transparent',
          display: 'flex',
          justifyContent: 'center',
          alignItems: 'center',
          marginTop: '-0.5rem',
          marginBottom: '-0.5rem',
          zIndex: 10
        }}
        title="Drag to resize"
      >
        <div style={{ width: '40px', height: '4px', backgroundColor: 'var(--border)', borderRadius: '2px' }}></div>
      </button>

      <div style={{ flex: 1, minHeight: '150px', backgroundColor: 'var(--bg-card)', border: '1px solid var(--border)', borderRadius: '8px', display: 'flex', flexDirection: 'column' }}>
        {selectedLog ? (
          <>
            <div style={{ display: 'flex', backgroundColor: 'var(--bg)', borderTopLeftRadius: '8px', borderTopRightRadius: '8px', borderBottom: '1px solid var(--border)' }}>
              <button
                className="btn"
                style={{ backgroundColor: 'transparent', color: logViewTab === 'inbound' ? 'var(--primary)' : 'var(--text-muted)', borderBottom: logViewTab === 'inbound' ? '2px solid var(--primary)' : '2px solid transparent', borderRadius: 0, padding: '0.75rem 1.5rem' }}
                onClick={() => setLogViewTab('inbound')}
              >
                Raw (Inbound)
              </button>
              <button
                className="btn"
                style={{ backgroundColor: 'transparent', color: logViewTab === 'outbound' ? 'var(--primary)' : 'var(--text-muted)', borderBottom: logViewTab === 'outbound' ? '2px solid var(--primary)' : '2px solid transparent', borderRadius: 0, padding: '0.75rem 1.5rem' }}
                onClick={() => setLogViewTab('outbound')}
              >
                Encoded (Outbound)
              </button>
              <button
                className="btn"
                style={{ backgroundColor: 'transparent', color: logViewTab === 'response' ? 'var(--primary)' : 'var(--text-muted)', borderBottom: logViewTab === 'response' ? '2px solid var(--primary)' : '2px solid transparent', borderRadius: 0, padding: '0.75rem 1.5rem' }}
                onClick={() => setLogViewTab('response')}
              >
                Target Response (ACK)
              </button>
            </div>
            <div style={{ flex: 1, padding: '0.5rem', overflow: 'hidden', display: 'flex' }}>
              <textarea
                readOnly
                value={getLogContent()}
                style={{
                  width: '100%',
                  height: '100%',
                  backgroundColor: '#000000',
                  color: '#A78BFA',
                  border: '1px solid var(--border)',
                  borderRadius: '4px',
                  resize: 'none',
                  fontFamily: 'Consolas, Monaco, monospace',
                  fontSize: '0.85rem',
                  padding: '1rem',
                  outline: 'none',
                  whiteSpace: 'pre-wrap'
                }}
              />
            </div>
          </>
        ) : (
          <div style={{ flex: 1, display: 'flex', alignItems: 'center', justifyContent: 'center', color: 'var(--text-muted)' }}>
            Select a message from the list above to view its contents.
          </div>
        )}
      </div>
    </div>
  );

  const handleAddUser = async () => {
    const username = prompt("Enter username:");
    if (!username) return;
    const password = prompt("Enter password:");
    if (!password) return;

    try {
      await fetch(`${API_BASE}/users`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username, password })
      });
      void fetchUsers();
    } catch (e) {
      console.error(e);
    }
  };

  const handleEditUser = async (user) => {
    const password = prompt(`Enter new password for ${user.username}:`);
    if (!password) return;

    try {
      await fetch(`${API_BASE}/users/${user.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ password })
      });
      alert("Password updated");
    } catch (e) {
      console.error(e);
    }
  };

  const handleToggleUser = async (user) => {
    try {
      await fetch(`${API_BASE}/users/${user.id}/status`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ active: !user.active })
      });
      void fetchUsers();
    } catch (e) {
      console.error(e);
    }
  };

  const renderUsersView = () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
        <button className="btn btn-primary" onClick={handleAddUser}><Plus size={16} /> Add User</button>
      </div>
      <div style={{ backgroundColor: 'var(--bg-card)', border: '1px solid var(--border)', borderRadius: '8px', overflow: 'hidden' }}>
        <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
          <thead style={{ backgroundColor: 'var(--bg-card)', borderBottom: '1px solid var(--border)' }}>
            <tr>
              <th style={{ padding: '0.75rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Username</th>
              <th style={{ padding: '0.75rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Status</th>
              <th style={{ padding: '0.75rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {users.map(u => (
              <tr key={u.id} style={{ borderBottom: '1px solid var(--border)' }}>
                <td style={{ padding: '0.75rem 1rem' }}>{u.username}</td>
                <td style={{ padding: '0.75rem 1rem' }}>
                  <span style={{
                    padding: '0.2rem 0.4rem', borderRadius: '4px', fontSize: '0.75rem', fontWeight: 'bold',
                    backgroundColor: u.active ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
                    color: u.active ? '#10B981' : '#EF4444'
                  }}>
                    {u.active ? 'Enabled' : 'Disabled'}
                  </span>
                </td>
                <td style={{ padding: '0.75rem 1rem', display: 'flex', gap: '0.5rem' }}>
                  <button className="btn" onClick={() => handleEditUser(u)}>Edit Password</button>
                  <button className="btn" onClick={() => handleToggleUser(u)}>{u.active ? 'Disable' : 'Enable'}</button>
                </td>
              </tr>
            ))}
            {users.length === 0 && (
              <tr><td colSpan="3" style={{ padding: '2rem', textAlign: 'center' }}>No users found.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );

  const fetchUsers = async () => {
    try {
      const res = await fetch(`${API_BASE}/users`);
      const data = await res.json();
      setUsers(Array.isArray(data) ? data : []);
    } catch (e) {
      console.error(e);
    }
  };

  const handleLogin = async (e) => {
    e.preventDefault();
    try {
      const res = await fetch(`${API_BASE}/login`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ username: loginUser, password: loginPass })
      });
      const data = await res.json();
      if (data.success) {
        setToken(data.token);
        localStorage.setItem('token', data.token);
        setLoginError('');
      } else {
        setLoginError(data.error || 'Login failed');
      }
    } catch (e) {
      console.error(e);
      setLoginError('Network error');
    }
  };

  const handleLogout = () => {
    if (editingRoute && isRouteDirty()) {
      const confirmLeave = window.confirm(
        `You have unsaved changes in channel "${editingRoute.name || 'Untitled'}". Discard changes and logout?`
      );
      if (!confirmLeave) return;
    }
    setEditingRoute(null);
    setInitialRouteSnapshot(null);
    setToken('');
    localStorage.removeItem('token');
  };

  if (!token) {
    return (
      <div className="login-screen">
        <form onSubmit={handleLogin} className="login-card">
          <div className="login-logo-wrap">
            <img src={faviconLogo} alt="Logo" className="login-logo-img" />
          </div>
          <h2 className="login-title">HealthConnect</h2>
          <p className="login-subtitle">Healthcare Interoperability & FHIR Platform</p>
          {loginError && <div style={{ color: '#EF4444', fontSize: '0.875rem', textAlign: 'center' }}>{loginError}</div>}
          <div>
            <label htmlFor="loginUser" style={{ display: 'block', marginBottom: '0.5rem', fontSize: '0.875rem' }}>Username</label>
            <input id="loginUser" type="text" className="form-control" value={loginUser} onChange={e => setLoginUser(e.target.value)} required />
          </div>
          <div>
            <label htmlFor="loginPass" style={{ display: 'block', marginBottom: '0.5rem', fontSize: '0.875rem' }}>Password</label>
            <input id="loginPass" type="password" className="form-control" value={loginPass} onChange={e => setLoginPass(e.target.value)} required />
          </div>
          <button type="submit" className="btn btn-primary" style={{ marginTop: '1rem' }}>Login</button>
        </form>
      </div>
    );
  }

  const handleAddVariable = async () => {
    const keyName = prompt("Enter Variable Key (e.g. SATUSEHAT_CLIENT_ID):");
    if (!keyName) return;
    const valueData = prompt(`Enter Value for ${keyName}:`);
    if (valueData === null) return; // allows empty string
    const isSecret = confirm("Is this a secret? (Value will be hidden in UI)");
    const description = prompt("Enter description (optional):") || '';

    try {
      await fetch(`${API_BASE}/variables`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ key_name: keyName, value_data: valueData, is_secret: isSecret, description })
      });
      void fetchVariables();
    } catch (e) {
      console.error(e);
    }
  };

  const handleEditVariable = async (v) => {
    const newValue = prompt(`Enter new value for ${v.key_name} (leave empty to keep current if secret):`, v.is_secret ? '' : v.value_data);
    if (newValue === null) return;
    
    try {
      await fetch(`${API_BASE}/variables/${v.id}`, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ 
          key_name: v.key_name, 
          value_data: newValue || v.value_data, 
          is_secret: v.is_secret, 
          description: v.description 
        })
      });
      void fetchVariables();
    } catch (e) {
      console.error(e);
    }
  };

  const handleDeleteVariable = async (id) => {
    if (!confirm("Are you sure you want to delete this variable?")) return;
    try {
      await fetch(`${API_BASE}/variables/${id}`, { method: 'DELETE' });
      void fetchVariables();
    } catch (e) {
      console.error(e);
    }
  };

  const renderVariablesView = () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem', padding: '0 1rem' }}>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', backgroundColor: 'rgba(79, 70, 229, 0.1)', padding: '1rem', borderRadius: '8px', border: '1px solid rgba(79, 70, 229, 0.2)' }}>
        <div style={{ color: 'var(--primary)' }}>
          <strong>Tip:</strong> Use <code>{"{{VARIABLE_NAME}}"}</code> in your Channel configurations (URL, Headers, or Body) to dynamically inject these values.
        </div>
        <button className="btn btn-primary" onClick={handleAddVariable}><Plus size={16} /> Add Variable</button>
      </div>
      
      <div style={{ backgroundColor: 'var(--bg-card)', border: '1px solid var(--border)', borderRadius: '8px', overflow: 'hidden' }}>
        <table className="data-table" style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left' }}>
          <thead style={{ backgroundColor: 'var(--bg-card)', borderBottom: '1px solid var(--border)' }}>
            <tr>
              <th style={{ padding: '0.75rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Key</th>
              <th style={{ padding: '0.75rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Value</th>
              <th style={{ padding: '0.75rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)' }}>Description</th>
              <th style={{ padding: '0.75rem 1rem', fontSize: '0.875rem', color: 'var(--text-muted)', textAlign: 'right' }}>Actions</th>
            </tr>
          </thead>
          <tbody>
            {variables.map(v => (
              <tr key={v.id} style={{ borderBottom: '1px solid var(--border)' }}>
                <td style={{ padding: '0.75rem 1rem', fontWeight: 'bold', fontFamily: 'monospace', color: '#4338ca' }}>{v.key_name}</td>
                <td style={{ padding: '0.75rem 1rem', fontFamily: 'monospace', color: v.is_secret ? '#ef4444' : 'inherit' }}>
                  {v.is_secret ? '********' : v.value_data}
                  {v.is_secret && <span style={{ marginLeft: '8px', fontSize: '10px', backgroundColor: '#fee2e2', color: '#ef4444', padding: '2px 6px', borderRadius: '12px' }}>SECRET</span>}
                </td>
                <td style={{ padding: '0.75rem 1rem', fontSize: '0.85rem' }}>{v.description}</td>
                <td style={{ padding: '0.75rem 1rem', display: 'flex', gap: '0.5rem', justifyContent: 'flex-end' }}>
                  <button className="btn" onClick={() => handleEditVariable(v)}>Edit</button>
                  <button className="btn" style={{ color: '#ef4444' }} onClick={() => handleDeleteVariable(v.id)}>Delete</button>
                </td>
              </tr>
            ))}
            {variables.length === 0 && (
              <tr><td colSpan="4" style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-muted)' }}>No global variables configured.</td></tr>
            )}
          </tbody>
        </table>
      </div>
    </div>
  );

  const fetchLicense = async () => {
    try {
      const res = await fetch(`${API_BASE}/license`);
      const data = await res.json();
      setLicenseInfo(data);
    } catch (e) {
      console.error("Failed to fetch license", e);
    }
  };

  const handleActivateLicense = async (e) => {
    e?.preventDefault();
    if (!licenseKeyInput.trim()) {
      setLicenseMsg({ text: 'Please paste the license key first', type: 'error' });
      return;
    }
    try {
      const res = await fetch(`${API_BASE}/license/activate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ license_key: licenseKeyInput.trim() })
      });
      const data = await res.json();
      if (res.ok && data.valid) {
        setLicenseMsg({ text: `License successfully activated for: ${data.customer}`, type: 'success' });
        setLicenseKeyInput('');
        void fetchLicense();
      } else {
        setLicenseMsg({ text: data.error || 'License activation failed. Please check the license key and Machine Code.', type: 'error' });
      }
    } catch (e) {
      setLicenseMsg({ text: 'Failed to connect to backend server: ' + e.message, type: 'error' });
    }
  };

  const copyMachineCode = async () => {
    if (licenseInfo?.machine_code) {
      try {
        await navigator.clipboard?.writeText(licenseInfo.machine_code);
        setCopiedCode(true);
        setTimeout(() => setCopiedCode(false), 2000);
      } catch (err) {
        console.error('Failed to copy machine code', err);
      }
    }
  };

  const renderLicenseView = () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem', maxWidth: '1000px' }}>
      <LicenseMachineCard
        machineCode={licenseInfo?.machine_code}
        copiedCode={copiedCode}
        onCopy={copyMachineCode}
      />
      <LicenseStatusCard
        licenseInfo={licenseInfo}
        totalRouteCount={routes.length}
        activeRouteCount={routes.filter(r => r.active !== false).length}
      />
      <LicenseActivationCard
        licenseKeyInput={licenseKeyInput}
        setLicenseKeyInput={setLicenseKeyInput}
        licenseMsg={licenseMsg}
        setLicenseMsg={setLicenseMsg}
        onActivate={handleActivateLicense}
        machineCode={licenseInfo?.machine_code}
      />
    </div>
  );

  const renderDashboard = () => {
    const totalReceived = Object.values(stats).reduce((acc, s) => acc + (s.received || 0), 0);
    const totalFiltered = Object.values(stats).reduce((acc, s) => acc + (s.filtered || 0), 0);
    const totalQueue = Object.values(stats).reduce((acc, s) => acc + (s.queue || 0), 0);
    const totalSent = Object.values(stats).reduce((acc, s) => acc + (s.sent || 0), 0);
    const totalError = Object.values(stats).reduce((acc, s) => acc + (s.error || 0), 0);
    const activeChannels = routes.filter(r => r.active !== false).length;

    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
        {/* KPI Cards Row */}
        <div className="stats-grid">
          <div className="kpi-card" style={{ '--kpi-accent': 'var(--primary)', '--kpi-bg': 'var(--primary-light)' }}>
            <div className="kpi-header">
              <span className="kpi-title">Active Channels</span>
              <div className="kpi-icon-wrap"><Layers size={18} /></div>
            </div>
            <div className="kpi-value">{activeChannels} <span style={{ fontSize: '0.9rem', color: 'var(--text-muted)', fontWeight: 500 }}>/ {routes.length}</span></div>
            <div className="kpi-footer">
              <span style={{ color: 'var(--success-dark)', fontWeight: 700, display: 'inline-flex', alignItems: 'center' }}>
                <ArrowUpRight size={14} /> 100%
              </span>
              <span>interoperability connectors ready</span>
            </div>
          </div>

          <div className="kpi-card" style={{ '--kpi-accent': 'var(--accent-cyan)', '--kpi-bg': 'var(--accent-cyan-light)' }}>
            <div className="kpi-header">
              <span className="kpi-title">Inbound Messages</span>
              <div className="kpi-icon-wrap" style={{ color: 'var(--accent-cyan)' }}><Network size={18} /></div>
            </div>
            <div className="kpi-value">{totalReceived}</div>
            <div className="kpi-footer">
              <span style={{ color: 'var(--accent-cyan)', fontWeight: 700 }}>HL7 / FHIR / JSON</span>
              <span>from HIS</span>
            </div>
          </div>

          <div className="kpi-card" style={{ '--kpi-accent': 'var(--success)', '--kpi-bg': 'var(--success-light)' }}>
            <div className="kpi-header">
              <span className="kpi-title">Outbound Sent</span>
              <div className="kpi-icon-wrap" style={{ color: 'var(--success)' }}><ShieldCheck size={18} /></div>
            </div>
            <div className="kpi-value" style={{ color: 'var(--success)' }}>{totalSent}</div>
            <div className="kpi-footer">
              <span style={{ color: 'var(--success-dark)', fontWeight: 700, display: 'inline-flex', alignItems: 'center' }}>
                <ArrowUpRight size={14} /> Successful
              </span>
              <span>to target endpoints</span>
            </div>
          </div>

          <div className="kpi-card" style={{ '--kpi-accent': 'var(--accent-red)', '--kpi-bg': 'var(--accent-red-light)' }}>
            <div className="kpi-header">
              <span className="kpi-title">Issues & Errors</span>
              <div className="kpi-icon-wrap" style={{ color: 'var(--accent-red)' }}><HeartPulse size={18} /></div>
            </div>
            <div className="kpi-value" style={{ color: totalError > 0 ? 'var(--accent-red)' : 'var(--text-main)' }}>{totalError}</div>
            <div className="kpi-footer">
              <span style={{ color: totalError > 0 ? 'var(--accent-red)' : 'var(--success)', fontWeight: 700 }}>
                {totalError === 0 ? '✓ Zero incidents' : 'Requires technical attention'}
              </span>
            </div>
          </div>
        </div>

        {/* Visual Charts & Status Breakdown (Pure SVG, Zero Library) */}
        <div className="dashboard-visuals-grid">
          {/* Channel Traffic Bar Chart */}
          <div className="chart-card">
            <div className="chart-card-header">
              <div>
                <h3 className="chart-card-title">Integration Data Flow Volume per Channel</h3>
                <p className="chart-card-subtitle">Comparison of Inbound vs Outbound messages</p>
              </div>
              <span style={{ fontSize: '0.72rem', fontWeight: 700, color: 'var(--primary)', background: 'var(--primary-light)', padding: '0.2rem 0.6rem', borderRadius: '999px' }}>
                Live Stream
              </span>
            </div>

            {routes.length === 0 ? (
              <div style={{ padding: '3rem', textAlign: 'center', color: 'var(--text-muted)' }}>No channels registered yet</div>
            ) : (
              <div style={{ display: 'flex', flexDirection: 'column', gap: '0.9rem', marginTop: '0.5rem' }}>
                {routes.map(r => {
                  const s = stats[r.id] || { received: 0, sent: 0, error: 0 };
                  const maxVal = Math.max(1, ...routes.map(item => stats[item.id]?.received || 0));
                  const percentReceived = Math.min(100, Math.round((s.received / maxVal) * 100));
                  const percentSent = Math.min(100, Math.round((s.sent / maxVal) * 100));

                  return (
                    <div key={r.id} style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem' }}>
                      <div style={{ display: 'flex', justifyContent: 'space-between', fontSize: '0.8rem' }}>
                        <span style={{ fontWeight: 700, color: 'var(--primary)' }}>{r.name}</span>
                        <span style={{ color: 'var(--text-secondary)', fontFamily: 'monospace', fontSize: '0.75rem' }}>
                          In: <b>{s.received}</b> | Out: <b>{s.sent}</b> | Err: <b style={{ color: s.error > 0 ? 'var(--accent-red)' : 'inherit' }}>{s.error}</b>
                        </span>
                      </div>
                      <div style={{ width: '100%', height: '8px', background: 'var(--bg-subtle)', borderRadius: '999px', overflow: 'hidden', display: 'flex', gap: '2px' }}>
                        <div style={{ width: `${percentReceived}%`, background: 'var(--primary)', height: '100%', borderRadius: '999px', transition: 'width 0.4s ease' }} title={`Inbound: ${s.received}`} />
                        <div style={{ width: `${percentSent}%`, background: 'var(--accent-red)', height: '100%', borderRadius: '999px', transition: 'width 0.4s ease' }} title={`Outbound: ${s.sent}`} />
                      </div>
                    </div>
                  );
                })}
              </div>
            )}
            <div style={{ display: 'flex', gap: '1.25rem', marginTop: '0.5rem', paddingTop: '0.75rem', borderTop: '1px solid var(--border-light)', fontSize: '0.75rem' }}>
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', color: 'var(--text-secondary)' }}>
                <span style={{ width: 10, height: 10, borderRadius: 2, background: 'var(--primary)' }}></span> Inbound (Navy)
              </span>
              <span style={{ display: 'inline-flex', alignItems: 'center', gap: '0.4rem', color: 'var(--text-secondary)' }}>
                <span style={{ width: 10, height: 10, borderRadius: 2, background: 'var(--accent-red)' }}></span> Outbound (Red)
              </span>
            </div>
          </div>

          {/* Status Donut Summary (Pure SVG) */}
          <div className="chart-card">
            <div className="chart-card-header">
              <div>
                <h3 className="chart-card-title">Execution Status Ratio</h3>
                <p className="chart-card-subtitle">Payload handling distribution</p>
              </div>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', padding: '1rem 0' }}>
              <svg width="150" height="150" viewBox="0 0 42 42" style={{ transform: 'rotate(-90deg)' }}>
                <circle cx="21" cy="21" r="15.915" fill="transparent" stroke="var(--bg-subtle)" strokeWidth="4.5" />
                {/* Sent (Green) */}
                <circle
                  cx="21" cy="21" r="15.915" fill="transparent"
                  stroke="var(--success)" strokeWidth="4.5"
                  strokeDasharray={`${totalReceived > 0 ? (totalSent / totalReceived) * 100 : 0} ${100 - (totalReceived > 0 ? (totalSent / totalReceived) * 100 : 0)}`}
                  strokeDashoffset="0"
                />
                {/* Filtered (Cyan) */}
                <circle
                  cx="21" cy="21" r="15.915" fill="transparent"
                  stroke="var(--accent-cyan)" strokeWidth="4.5"
                  strokeDasharray={`${totalReceived > 0 ? (totalFiltered / totalReceived) * 100 : 0} ${100 - (totalReceived > 0 ? (totalFiltered / totalReceived) * 100 : 0)}`}
                  strokeDashoffset={`-${totalReceived > 0 ? (totalSent / totalReceived) * 100 : 0}`}
                />
                {/* Error (Red) */}
                <circle
                  cx="21" cy="21" r="15.915" fill="transparent"
                  stroke="var(--accent-red)" strokeWidth="4.5"
                  strokeDasharray={`${totalReceived > 0 ? (totalError / totalReceived) * 100 : 0} ${100 - (totalReceived > 0 ? (totalError / totalReceived) * 100 : 0)}`}
                  strokeDashoffset={`-${totalReceived > 0 ? ((totalSent + totalFiltered) / totalReceived) * 100 : 0}`}
                />
              </svg>
              <div style={{ marginTop: '-88px', marginBottom: '45px', textAlign: 'center', transform: 'none' }}>
                <div style={{ fontSize: '1.25rem', fontWeight: 800, color: 'var(--primary)' }}>{totalReceived}</div>
                <div style={{ fontSize: '0.65rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>Total</div>
              </div>
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '0.4rem', fontSize: '0.78rem' }}>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                  <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--success)' }}></span> Sent
                </span>
                <span style={{ fontWeight: 700 }}>{totalSent}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                  <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--accent-cyan)' }}></span> Filtered
                </span>
                <span style={{ fontWeight: 700 }}>{totalFiltered}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                  <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--accent-red)' }}></span> Error
                </span>
                <span style={{ fontWeight: 700, color: totalError > 0 ? 'var(--accent-red)' : 'inherit' }}>{totalError}</span>
              </div>
              <div style={{ display: 'flex', justifyContent: 'space-between' }}>
                <span style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                  <span style={{ width: 8, height: 8, borderRadius: '50%', background: 'var(--warning)' }}></span> Queued
                </span>
                <span style={{ fontWeight: 700 }}>{totalQueue}</span>
              </div>
            </div>
          </div>
        </div>
      </div>
    );
  };

  const renderContent = () => {
    if (loading) return <p>Loading...</p>;
    if (activeTab === 'dashboard') return renderDashboard();
    if (activeTab === 'integrations') return renderIntegrations();
    if (activeTab === 'license') return renderLicenseView();
    if (activeTab === 'users') return renderUsersView();
    if (activeTab === 'environments') return renderVariablesView();
    return renderLogsView();
  };


  const isSelectedActive = () => {
    const route = routes.find(r => r.id === selectedRouteId);
    if (!route) return true;
    return route.active !== false;
  };

  return (
    <div className="app-layout">
      {isSidebarOpen && (
        <button 
          className="sidebar-overlay" 
          aria-label="Close sidebar" 
          onClick={() => setIsSidebarOpen(false)} 
          style={{ border: 'none', padding: 0, margin: 0 }}
        />
      )}
      <div className={`sidebar ${isSidebarOpen ? 'open' : ''}`}>
        <div className="sidebar-header">
          <img src={faviconLogo} alt="Logo" className="brand-logo-img" />
          <div className="brand-info">
            <span className="brand-title">HealthConnect</span>
            <span className="brand-tag">Hospital Interoperability</span>
          </div>
        </div>
        <nav className="sidebar-menu main-nav">
          <button className={(!editingRoute && activeTab === 'dashboard') ? 'active' : ''} onClick={() => navigateToTab('dashboard')}>
            <LayoutDashboard size={16} /> Dashboard
          </button>
          <button className={(editingRoute || activeTab === 'integrations') ? 'active' : ''} onClick={() => navigateToTab('integrations')}>
            <GitMerge size={16} /> Channels
          </button>
          <button className={(!editingRoute && activeTab === 'environments') ? 'active' : ''} onClick={() => navigateToTab('environments')}>
            <KeyRound size={16} /> Environments
          </button>
          <button className={(!editingRoute && activeTab === 'license') ? 'active' : ''} onClick={() => navigateToTab('license')}>
            <Award size={16} /> License & Machine
          </button>
          <button className={(!editingRoute && activeTab === 'users') ? 'active' : ''} onClick={() => navigateToTab('users')}>
            <UsersIcon size={16} /> Users
          </button>
        </nav>

        {editingRoute ? (
          <>
            <div className="sidebar-section">Active Channel</div>
            <nav className="sidebar-menu">
              <div style={{ padding: '0.4rem 0.6rem', fontSize: '0.8rem', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '0.5rem' }}>
                <span style={{ fontWeight: 600, color: 'var(--primary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }} title={editingRoute.name}>
                  ✏️ {editingRoute.name || 'Untitled'}
                </span>
                {isRouteDirty() && (
                  <span style={{ fontSize: '0.65rem', color: '#D97706', backgroundColor: 'rgba(217, 119, 6, 0.1)', padding: '0.1rem 0.4rem', borderRadius: '4px', fontWeight: 700, flexShrink: 0 }}>
                    MODIFIED
                  </span>
                )}
              </div>
              <button onClick={saveRoute} style={{ color: 'var(--primary)', fontWeight: 600, display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                <Save size={15} /> Save Channel
              </button>
              <button onClick={handleCloseEditor} style={{ display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
                <ArrowLeft size={15} /> Back to Channels
              </button>
            </nav>
          </>
        ) : activeTab === 'integrations' ? (
          <>
            <div className="sidebar-section">Channel Tasks</div>
            <nav className="sidebar-menu">
              <button onClick={createRoute}>New Channel</button>
              <button disabled={!selectedRouteId} onClick={() => selectedRouteId && cloneRoute(selectedRouteId)}>
                Clone Channel
              </button>
              <button disabled={!selectedRouteId} onClick={() => {
                if (selectedRouteId) openEdit(routes.find(r => r.id === selectedRouteId));
              }}>
                Edit Channel
              </button>
              <button disabled={!selectedRouteId} onClick={() => selectedRouteId && toggleRoute(selectedRouteId)}>
                {isSelectedActive() ? 'Disable Channel' : 'Enable Channel'}
              </button>
              <button
                disabled={!selectedRouteId}
                style={{ color: '#ef4444' }}
                onClick={() => selectedRouteId && deleteRoute(selectedRouteId)}
                title={licenseInfo?.is_trial ? 'Channels cannot be deleted during Trial mode' : 'Delete Channel'}
              >
                Delete Channel
              </button>
            </nav>

            <div className="sidebar-section">Group Tasks</div>
            <nav className="sidebar-menu">
              <button onClick={createGroup}>New Group</button>
              <button disabled={!selectedRouteId} onClick={() => {
                if (selectedRouteId) openEdit(routes.find(r => r.id === selectedRouteId));
              }}>
                Assign To Group
              </button>
            </nav>
          </>
        ) : null}

        <div className="sidebar-section">Other</div>
        <nav className="sidebar-menu" style={{ marginTop: 'auto', marginBottom: '1rem' }}>
          <button onClick={handleLogout}><LogOut size={16} /> Logout</button>
        </nav>
      </div>

      <div className="main-content">
        {editingRoute ? (
          <ChannelEditor
            editingRoute={editingRoute}
            setEditingRoute={setEditingRoute}
            routes={routes}
            groups={groups}
            testStatus={testStatus}
            testConnection={testConnection}
            saveRoute={saveRoute}
            onClose={handleCloseEditor}
            isDirty={isRouteDirty()}
          />
        ) : (
          <>
            <div className="page-header">
              <div style={{ display: 'flex', alignItems: 'center', gap: '1rem' }}>
                <button 
                  className="mobile-menu-btn" 
                  onClick={() => setIsSidebarOpen(true)}
                  style={{ background: 'none', border: 'none', cursor: 'pointer', padding: 0 }}
                >
                  <Menu size={24} color="var(--primary)" />
                </button>
                <h1 className="page-title">
                  {{ 'integrations': 'Channels', 'logs': 'Message Logs', 'users': 'Users', 'environments': 'Global Environments', 'license': 'License & Machine Management' }[activeTab] || 'Dashboard'}
                </h1>
              </div>
            </div>
            {renderContent()}
          </>
        )}
      </div>
    </div>
  );
}

export default App;
