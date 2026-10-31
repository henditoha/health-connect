import React, { useState, useEffect } from 'react';
import { ArrowLeft, Save, Play, Filter, Trash2, Plus, Eye, EyeOff } from './icons';

function VariablesEditor({
  title,
  description,
  variables = {},
  onChange,
  addLabel = "+ Add Variable",
  varPrefix = "var",
  visibleVars = {},
  onToggleVisibility
}) {
  const entries = Object.entries(variables || {});

  const handleKeyChange = (oldKey, newKey, val) => {
    const next = {};
    for (const [k, v] of Object.entries(variables || {})) {
      if (k === oldKey) {
        next[newKey] = val;
      } else {
        next[k] = v;
      }
    }
    onChange(next);
  };

  const handleValueChange = (key, val) => {
    onChange({
      ...variables,
      [key]: val
    });
  };

  const handleRemove = (key) => {
    const next = { ...variables };
    delete next[key];
    onChange(next);
  };

  const handleAdd = () => {
    const newKey = `VAR_${Date.now().toString().slice(-4)}`;
    onChange({
      ...variables,
      [newKey]: ''
    });
  };

  return (
    <div className="editor-card">
      <h3 style={{ margin: '0 0 0.5rem 0', color: 'var(--primary)', fontSize: '1.1rem' }}>
        {title}
      </h3>
      <p style={{ fontSize: '0.875rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
        {description}
      </p>

      <div style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}>
        {entries.length === 0 && (
          <div style={{ fontSize: '0.85rem', color: 'var(--text-muted)', fontStyle: 'italic', padding: '0.5rem 0' }}>
            No scoped variables defined for this endpoint.
          </div>
        )}
        {entries.map(([key, val]) => {
          const isSecret = /key|pass|secret|token|cred|auth|pwd|dsn/i.test(key);
          const varId = `${varPrefix}_${key}`;
          const isVisible = !!visibleVars[varId];

          return (
            <div key={key} style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
              <input
                className="form-control"
                style={{ flex: 1, fontFamily: 'monospace', fontWeight: 600, color: '#4338CA' }}
                placeholder="VARIABLE_NAME"
                value={key}
                onChange={e => handleKeyChange(key, e.target.value, val)}
              />
              <div style={{ flex: 2, display: 'flex', alignItems: 'center', position: 'relative' }}>
                <input
                  className="form-control"
                  style={{ width: '100%', fontFamily: 'monospace', paddingRight: isSecret ? '36px' : '10px' }}
                  type={isSecret && !isVisible ? 'password' : 'text'}
                  placeholder="Value or secret..."
                  value={val || ''}
                  onChange={e => handleValueChange(key, e.target.value)}
                />
                {isSecret && (
                  <button
                    type="button"
                    onClick={() => onToggleVisibility?.(varId)}
                    style={{
                      position: 'absolute',
                      right: '8px',
                      background: 'none',
                      border: 'none',
                      cursor: 'pointer',
                      padding: 0,
                      color: 'var(--text-muted)',
                      display: 'flex',
                      alignItems: 'center'
                    }}
                    title={isVisible ? 'Hide value' : 'Show value'}
                  >
                    {isVisible ? <EyeOff size={16} /> : <Eye size={16} />}
                  </button>
                )}
              </div>
              <button className="btn btn-danger" onClick={() => handleRemove(key)}>
                Remove
              </button>
            </div>
          );
        })}
        <button className="btn" onClick={handleAdd}>
          {addLabel}
        </button>
      </div>
    </div>
  );
}

const API_BASE = import.meta.env.DEV ? 'http://localhost:3001/api' : '/api';

export default function ChannelEditor({
  editingRoute,
  setEditingRoute,
  routes = [],
  groups = [],
  testStatus = {},
  testConnection,
  saveRoute,
  onClose,
  isDirty = false
}) {
  const [visibleVars, setVisibleVars] = useState({});
  const [registry, setRegistry] = useState({ protocols: [], database_drivers: [], formats: [] });
  const [savedConnectors, setSavedConnectors] = useState([]);

  useEffect(() => {
    fetch(`${API_BASE}/connector-registry`)
      .then(res => res.json())
      .then(data => {
        if (data && data.protocols) setRegistry(data);
      })
      .catch(() => {});

    fetch(`${API_BASE}/connectors`)
      .then(res => res.json())
      .then(data => {
        if (Array.isArray(data)) setSavedConnectors(data);
      })
      .catch(() => {});
  }, []);

  const handleSaveAsConnector = async (role) => {
    const isSource = role === 'source';
    const defaultName = isSource ? `${editingRoute.name} Source` : (activeDest.config?.name || `${editingRoute.name} Target`);
    const connName = prompt(`Enter a name for this reusable ${isSource ? 'source' : 'target'} connector profile:`, defaultName);
    if (!connName) return;

    const payload = {
      name: connName,
      role: isSource ? 'source' : 'target',
      protocol: isSource ? editingRoute.source.protocol : activeDest.protocol,
      format: isSource ? editingRoute.source.format : activeDest.format,
      config: isSource ? (editingRoute.source.config || {}) : (activeDest.config || {}),
      description: `Saved from channel: ${editingRoute.name}`
    };

    try {
      const res = await fetch(`${API_BASE}/connectors`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload)
      });
      if (res.ok) {
        alert(`Connector profile "${connName}" saved successfully!`);
        fetch(`${API_BASE}/connectors`)
          .then(r => r.json())
          .then(data => { if (Array.isArray(data)) setSavedConnectors(data); });
      } else {
        const err = await res.json();
        alert('Failed: ' + (err.error || 'Unknown error'));
      }
    } catch (err) {
      alert('Network error: ' + err.message);
    }
  };

  useEffect(() => {
    const handleKeyDown = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 's') {
        e.preventDefault();
        saveRoute();
      } else if (e.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [saveRoute, onClose]);

  if (!editingRoute) return null;

  const targets = editingRoute.targets || [];
  const activeDestIdx = Math.min(editingRoute.activeTargetIdx || 0, Math.max(0, targets.length - 1));
  const activeDest = targets[activeDestIdx] || targets[0] || { protocol: 'http', format: 'json', config: {} };
  const sourceTest = testStatus?.['source'];
  const destTest = testStatus?.[`target_${activeDestIdx}`];

  const updateSourceConfig = (patch) => {
    setEditingRoute(prev => ({
      ...prev,
      source: {
        ...prev.source,
        config: {
          ...prev.source?.config,
          ...patch
        }
      }
    }));
  };

  const addDestination = () => {
    const nextNum = targets.length + 1;
    const newDest = {
      id: `dest_${Date.now()}`,
      protocol: 'http',
      format: 'json',
      config: {
        name: `Destination ${nextNum}`,
        enabled: true,
        host: '',
        port: '',
        path: '',
        authType: 'none',
        filter_script: '',
        transformer_script: '',
        outbound_template: '',
        variables: {}
      }
    };
    setEditingRoute({
      ...editingRoute,
      targets: [...targets, newDest],
      activeTargetIdx: targets.length
    });
  };

  const removeDestination = (idx) => {
    if (targets.length <= 1) {
      alert("A channel must have at least one destination.");
      return;
    }
    const destName = targets[idx]?.config?.name || `Destination ${idx + 1}`;
    if (window.confirm(`Delete destination "${destName}"?`)) {
      const updated = targets.filter((_, i) => i !== idx);
      setEditingRoute({
        ...editingRoute,
        targets: updated,
        activeTargetIdx: Math.max(0, activeDestIdx - (activeDestIdx >= idx ? 1 : 0))
      });
    }
  };

  const updateActiveDest = (updater) => {
    const updatedTargets = [...targets];
    updatedTargets[activeDestIdx] = typeof updater === 'function' ? updater(updatedTargets[activeDestIdx]) : updater;
    setEditingRoute({ ...editingRoute, targets: updatedTargets });
  };

  const updateActiveDestConfig = (cfgUpdater) => {
    const updatedTargets = [...targets];
    const current = updatedTargets[activeDestIdx] || {};
    const currCfg = current.config || {};
    const newCfg = typeof cfgUpdater === 'function' ? cfgUpdater(currCfg) : { ...currCfg, ...cfgUpdater };
    updatedTargets[activeDestIdx] = { ...current, config: newCfg };
    setEditingRoute({ ...editingRoute, targets: updatedTargets });
  };

  return (
    <div className="channel-editor-container">
      {/* Top Header & Action Bar */}
      <div className="channel-editor-header">
        <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', flex: 1 }}>
          <button 
            className="btn btn-outline" 
            onClick={onClose}
            style={{ padding: '0.45rem 0.9rem', display: 'flex', alignItems: 'center', gap: '0.4rem', fontWeight: 600 }}
          >
            <ArrowLeft size={16} /> Channels
          </button>
          <div style={{ display: 'flex', flexDirection: 'column', flex: 1, marginRight: '1rem' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem' }}>
              <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)', textTransform: 'uppercase', letterSpacing: '0.5px' }}>
                Channel Editor
              </span>
              {isDirty && (
                <span style={{ fontSize: '0.7rem', color: '#D97706', backgroundColor: 'rgba(217, 119, 6, 0.1)', border: '1px solid rgba(217, 119, 6, 0.25)', padding: '0.15rem 0.5rem', borderRadius: '999px', fontWeight: 600 }}>
                  ● Unsaved Changes
                </span>
              )}
            </div>
            <input
              value={editingRoute.name}
              onChange={e => setEditingRoute({ ...editingRoute, name: e.target.value })}
              placeholder="Enter channel name..."
              style={{ fontSize: '1.4rem', fontWeight: 'bold', background: 'transparent', border: 'none', color: 'var(--text)', outline: 'none', borderBottom: '1px dashed var(--border)', padding: '0.1rem 0' }}
            />
          </div>
        </div>
        <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
          <button className="btn btn-secondary" onClick={onClose} style={{ padding: '0.6rem 1.1rem' }} title="Cancel and return to Channels (Esc)">
            Cancel
          </button>
          <button className="btn btn-primary" onClick={saveRoute} style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', padding: '0.6rem 1.25rem', fontWeight: 'bold' }} title="Save Channel (Ctrl+S)">
            <Save size={18} /> Save Channel
          </button>
        </div>
      </div>

      {/* Channel Summary Card */}
      <div className="editor-card" style={{ display: 'flex', gap: '1.5rem', flexWrap: 'wrap', alignItems: 'center' }}>
        <div className="form-group" style={{ flex: 1, minWidth: '220px', margin: 0 }}>
          <label htmlFor="group-name-input">Group Name</label>
          <select id="group-name-input" className="form-control" value={editingRoute.group_name || ''} onChange={e => setEditingRoute({ ...editingRoute, group_name: e.target.value })}>
            <option value="">-- Ungrouped --</option>
            {groups.map(g => <option key={g.id} value={g.name}>{g.name}</option>)}
          </select>
        </div>
        <div className="form-group" style={{ flex: 2, minWidth: '300px', margin: 0 }}>
          <label htmlFor="description-input">Description</label>
          <input id="description-input" className="form-control" value={editingRoute.description || ''} onChange={e => setEditingRoute({ ...editingRoute, description: e.target.value })} placeholder="Channel description & purpose..." />
        </div>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '0.25rem', minWidth: '140px' }}>
          <span style={{ fontSize: '0.875rem', fontWeight: 500, color: 'var(--text-muted)' }}>Channel Status</span>
          <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', height: '40px' }}>
            <span style={{ width: '10px', height: '10px', borderRadius: '50%', backgroundColor: editingRoute.active !== false ? '#10B981' : '#6B7280' }} />
            <span style={{ fontWeight: 600, color: editingRoute.active !== false ? '#10B981' : '#6B7280' }}>
              {editingRoute.active !== false ? 'Enabled' : 'Disabled'}
            </span>
          </div>
        </div>
      </div>

      {/* Navigation Tabs */}
      <div className="channel-nav-tabs">
        <button 
          className={`channel-nav-tab ${editingRoute.channelTab === 'source' ? 'active' : ''}`}
          onClick={() => setEditingRoute({ ...editingRoute, channelTab: 'source' })}
        >
          📥 Source Connector & Filter
        </button>
        <button 
          className={`channel-nav-tab ${editingRoute.channelTab === 'destinations' ? 'active' : ''}`}
          onClick={() => setEditingRoute({ ...editingRoute, channelTab: 'destinations' })}
        >
          📤 Destinations ({targets.length})
        </button>
      </div>

      {/* ==================== SOURCE TAB ==================== */}
      {editingRoute.channelTab === 'source' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          {/* Source Config Card */}
          <div className="editor-card">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.25rem', borderBottom: '1px solid var(--border)', paddingBottom: '0.75rem' }}>
              <h3 style={{ margin: 0, color: 'var(--primary)', fontSize: '1.1rem', display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                ⚙️ Source Configuration
              </h3>
              <div style={{ display: 'flex', gap: '0.5rem', alignItems: 'center' }}>
                {sourceTest?.state === 'loading' && <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>Testing... ⏳</span>}
                {sourceTest?.state === 'success' && <span style={{ fontSize: '0.85rem', color: '#10B981', fontWeight: 600 }}>✅ {sourceTest.message}</span>}
                {sourceTest?.state === 'error' && <span style={{ fontSize: '0.85rem', color: '#EF4444', fontWeight: 600 }}>❌ {sourceTest.message}</span>}
                <button 
                  className="btn btn-test" 
                  onClick={() => testConnection('source')} 
                  style={{ padding: '0.45rem 1rem', fontSize: '0.85rem', display: 'flex', alignItems: 'center', gap: '0.4rem', cursor: 'pointer' }}
                >
                  <Play size={14} /> Test Connection
                </button>
              </div>
            </div>

            {/* Reusable Connector Loader Bar */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.25rem', padding: '0.6rem 0.9rem', backgroundColor: 'var(--bg)', borderRadius: '8px', border: '1px solid var(--border)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', flex: 1, maxWidth: '480px' }}>
                <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--primary)', whiteSpace: 'nowrap' }}>⚡ Saved Connector:</span>
                <select
                  className="form-control"
                  style={{ padding: '0.35rem 0.6rem', fontSize: '0.85rem' }}
                  onChange={(e) => {
                    const conn = savedConnectors.find(c => c.id === e.target.value);
                    if (conn) {
                      setEditingRoute({
                        ...editingRoute,
                        source: {
                          ...editingRoute.source,
                          protocol: conn.protocol,
                          format: conn.format || editingRoute.source.format,
                          config: { ...(editingRoute.source.config || {}), ...(conn.config || {}) }
                        }
                      });
                    }
                  }}
                  defaultValue=""
                >
                  <option value="">-- Load from Reusable Connector Profile --</option>
                  {savedConnectors.filter(c => c.role === 'source' || c.role === 'shared').map(c => (
                    <option key={c.id} value={c.id}>{c.name} ({c.protocol?.toUpperCase()})</option>
                  ))}
                </select>
              </div>
              <button
                type="button"
                className="btn btn-outline"
                style={{ padding: '0.35rem 0.75rem', fontSize: '0.8rem' }}
                onClick={() => handleSaveAsConnector('source')}
              >
                💾 Save as Profile
              </button>
            </div>

            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: '1rem' }}>
              <div className="form-group">
                <label htmlFor="source-protocol">Protocol / Method</label>
                <select
                  id="source-protocol"
                  className="form-control"
                  value={editingRoute.source.protocol}
                  onChange={e => setEditingRoute({ ...editingRoute, source: { ...editingRoute.source, protocol: e.target.value } })}
                >
                  {(registry.protocols.length > 0 ? registry.protocols : [
                    { code: 'http', name: 'HTTP POST Ingress' },
                    { code: 'http-poll', name: 'HTTP GET Poll' },
                    { code: 'tcp', name: 'TCP Server' },
                    { code: 'database', name: 'Database Poll' },
                    { code: 'internal', name: 'Internal Queue Reader' }
                  ]).map(p => (
                    <option key={p.code} value={p.code}>{p.name}</option>
                  ))}
                </select>
              </div>
              <div className="form-group">
                <label htmlFor="source-format">Data Format</label>
                <select
                  id="source-format"
                  className="form-control"
                  value={editingRoute.source.format}
                  onChange={e => setEditingRoute({ ...editingRoute, source: { ...editingRoute.source, format: e.target.value } })}
                >
                  {(registry.formats.length > 0 ? registry.formats : [
                    { code: 'json', name: 'JSON' },
                    { code: 'xml', name: 'XML' },
                    { code: 'hl7', name: 'HL7 v2' },
                    { code: 'fhir', name: 'FHIR' },
                    { code: 'raw', name: 'Raw String' }
                  ]).map(f => (
                    <option key={f.code} value={f.code}>{f.name}</option>
                  ))}
                </select>
              </div>
              <div className="form-group">
                <label htmlFor="source-ack">ACK Behavior</label>
                <select
                  id="source-ack"
                  className="form-control"
                  value={editingRoute.source.config?.ackBehavior || 'deferred'}
                  onChange={e => updateSourceConfig({ ackBehavior: e.target.value })}
                >
                  <option value="deferred">Wait for Target Response (Deferred ACK)</option>
                  <option value="immediate">Immediate ACK (Process in background)</option>
                </select>
              </div>
            </div>

            {/* Protocol Specific Inputs */}
            {editingRoute.source.protocol === 'http' && (
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                <label htmlFor="source-http-url">Ingress URL (Read-only)</label>
                <input id="source-http-url" type="text" className="form-control" readOnly
                  value={`http://localhost:3001/ingress/${editingRoute.id}`}
                  style={{ backgroundColor: 'var(--bg)', color: 'var(--success)', fontFamily: 'monospace' }}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '4px' }}>
                  External systems can send POST requests to this URL to trigger this integration.
                </div>
              </div>
            )}

            {editingRoute.source.protocol === 'tcp' && (
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                <label htmlFor="source-tcp-port">Listen Port</label>
                <input id="source-tcp-port" type="text" className="form-control" placeholder="e.g. 9000"
                  value={editingRoute.source.config?.port || ''}
                  onChange={e => updateSourceConfig({ port: e.target.value })}
                />
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '4px' }}>
                  The engine starts a TCP socket server on this port to receive incoming MLLP/HL7 messages.
                </div>
              </div>
            )}

            {editingRoute.source.protocol === 'http-poll' && (
              <div style={{ marginTop: '0.5rem' }}>
                <div style={{ display: 'grid', gridTemplateColumns: '150px 1fr', gap: '1rem' }}>
                  <div className="form-group">
                    <label htmlFor="source-poll-interval">Interval (sec)</label>
                    <input id="source-poll-interval" type="number" className="form-control" placeholder="60" min="1"
                      value={editingRoute.source.config?.pollInterval || ''}
                      onChange={e => updateSourceConfig({ pollInterval: e.target.value })}
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="source-http-poll-url">API URL to Poll (GET)</label>
                    <input id="source-http-poll-url" type="text" className="form-control" placeholder="e.g. https://api.hospital.com/data?status=new"
                      value={editingRoute.source.config?.url || ''}
                      onChange={e => updateSourceConfig({ url: e.target.value })}
                    />
                  </div>
                </div>
                <div className="form-group">
                  <label htmlFor="source-http-poll-headers">Custom Headers (JSON format)</label>
                  <textarea id="source-http-poll-headers" className="form-control" rows="2" placeholder='{ "x-api-key": "{{API_KEY}}", "Authorization": "Bearer {{TOKEN}}" }'
                    value={editingRoute.source.config?.headers || ''}
                    onChange={e => updateSourceConfig({ headers: e.target.value })}
                  />
                </div>
              </div>
            )}

            {editingRoute.source.protocol === 'database' && (
              <div style={{ marginTop: '0.5rem' }}>
                <div style={{ display: 'grid', gridTemplateColumns: '150px 220px 1fr', gap: '1rem' }}>
                  <div className="form-group">
                    <label htmlFor="source-db-interval">Interval (sec)</label>
                    <input id="source-db-interval" type="number" className="form-control" placeholder="60" min="1"
                      value={editingRoute.source.config?.pollInterval || ''}
                      onChange={e => updateSourceConfig({ pollInterval: e.target.value })}
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="source-db-driver">Database Driver</label>
                    <select id="source-db-driver" className="form-control"
                      value={editingRoute.source.config?.driver || 'mysql'}
                      onChange={e => updateSourceConfig({ driver: e.target.value })}
                    >
                      {(registry.database_drivers.length > 0 ? registry.database_drivers : [
                        { code: 'mysql', name: 'MySQL' },
                        { code: 'mariadb', name: 'MariaDB' },
                        { code: 'postgres', name: 'PostgreSQL' },
                        { code: 'sqlserver', name: 'SQL Server (MSSQL)' },
                        { code: 'sqlite', name: 'SQLite' },
                        { code: 'hdb', name: 'SAP HANA' },
                        { code: 'odbc', name: 'ODBC (Generic)' }
                      ]).map(d => (
                        <option key={d.code} value={d.code}>{d.name}</option>
                      ))}
                    </select>
                  </div>
                  <div className="form-group">
                    <label htmlFor="source-db-dsn">Connection String (DSN)</label>
                    <input id="source-db-dsn" type="text" className="form-control" placeholder="user:pass@tcp(localhost:3306)/dbname or postgres://user:pass@host/db"
                      value={editingRoute.source.config?.dsn || ''}
                      onChange={e => updateSourceConfig({ dsn: e.target.value })}
                    />
                  </div>
                </div>
                <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem' }}>
                  <div className="form-group">
                    <label htmlFor="source-db-query">Select Query (Poller)</label>
                    <textarea id="source-db-query" className="form-control" rows="2" placeholder="SELECT id, patient_id FROM orders WHERE status='new'"
                      value={editingRoute.source.config?.query || ''}
                      onChange={e => updateSourceConfig({ query: e.target.value })}
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor="source-db-post-query">Post-Process Update Query</label>
                    <textarea id="source-db-post-query" className="form-control" rows="2" placeholder="UPDATE orders SET status='processed' WHERE status='new'"
                      value={editingRoute.source.config?.postQuery || ''}
                      onChange={e => updateSourceConfig({ postQuery: e.target.value })}
                    />
                  </div>
                </div>
              </div>
            )}
          </div>

          {/* Source Filter Card */}
          <div className="filter-card">
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <Filter size={18} color="#10B981" />
                <h4 style={{ margin: 0, fontSize: '1rem', color: '#10B981', fontWeight: 600 }}>Source Filter (Inbound Filter Rule)</h4>
                <span className="filter-badge">Pre-Routing</span>
              </div>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                Evaluated before processing. Return <code>true</code> to pass, <code>false</code> to drop.
              </div>
            </div>
            <textarea
              className="form-control editor-textarea"
              style={{ fontSize: '12px', minHeight: '100px', borderLeft: '3px solid #10B981', backgroundColor: '#0d1117', color: '#56d364' }}
              placeholder="// JavaScript filter condition. Example:&#10;return msg.status !== 'CANCELLED';&#10;// or:&#10;return JSON.parse(input).type === 'LAB';"
              value={editingRoute.source.config?.filter_script || ''}
              onChange={e => updateSourceConfig({ filter_script: e.target.value })}
            />
          </div>

          {/* Source Inbound Transformer Card */}
          <div className="editor-card">
            <h3 style={{ margin: '0 0 0.5rem 0', color: 'var(--primary)', fontSize: '1.1rem' }}>
              🔄 Source Inbound Transformer & Data Enrichment
            </h3>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
              Pre-process, normalize, or enrich incoming data. Available functions: <code>http.request()</code>, <code>cache.get/set()</code>, <code>getGlobalVar("KEY")</code>, and <code>dbQuery()</code>.
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem' }}>
              <div className="form-group" style={{ margin: 0 }}>
                <label htmlFor="inbound-template" style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>Inbound Data Template</span>
                  <span style={{ fontSize: '10px', color: '#818CF8' }}>Sample Inbound Message</span>
                </label>
                <textarea id="inbound-template" className="form-control editor-textarea"
                  style={{ fontSize: '12px', minHeight: '260px' }}
                  placeholder="Paste sample incoming payload (JSON, HL7, XML) for reference..."
                  value={editingRoute.source.config?.inbound_template || ''}
                  onChange={e => updateSourceConfig({ inbound_template: e.target.value })}
                />
              </div>
              <div className="form-group" style={{ margin: 0 }}>
                <label htmlFor="source-transformer-script" style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>Source Transformer Script (JavaScript)</span>
                  <span style={{ fontSize: '10px', color: '#818CF8' }}>Inbound Preprocessor</span>
                </label>
                <textarea id="source-transformer-script" className="form-control editor-textarea"
                  style={{ fontSize: '12px', minHeight: '260px', borderLeft: '2px solid #818CF8' }}
                  placeholder="// Optional: Pre-process or normalize raw incoming data before routing&#10;var data = JSON.parse(input);&#10;return JSON.stringify(data);"
                  value={editingRoute.source.config?.transformer_script || ''}
                  onChange={e => updateSourceConfig({ transformer_script: e.target.value })}
                />
              </div>
            </div>
          </div>

          {/* Source Scoped Variables Card */}
          <VariablesEditor
            title="🔑 Source Variables"
            description={<>Define variables used by Source (e.g. <code>SOURCE_API_KEY</code>, <code>DB_USER</code>). Reference with <code>{"{{KEY_NAME}}"}</code> in URLs, headers, or DSNs.</>}
            variables={editingRoute.source?.variables || {}}
            onChange={next => setEditingRoute({ ...editingRoute, source: { ...editingRoute.source, variables: next } })}
            addLabel="+ Add Source Variable"
            varPrefix="source"
            visibleVars={visibleVars}
            onToggleVisibility={k => setVisibleVars(prev => ({ ...prev, [k]: !prev[k] }))}
          />
        </div>
      )}

      {/* ==================== DESTINATIONS TAB (MULTI-DESTINATION) ==================== */}
      {editingRoute.channelTab === 'destinations' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '1.5rem' }}>
          {/* Destination Selection Bar */}
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', backgroundColor: 'var(--bg-card)', padding: '0.75rem 1rem', borderRadius: '8px', border: '1px solid var(--border)' }}>
            <div className="destination-pills-bar" style={{ margin: 0 }}>
              {targets.map((tgt, idx) => (
                <button
                  key={tgt.id || idx}
                  className={`destination-pill ${activeDestIdx === idx ? 'active' : ''}`}
                  onClick={() => setEditingRoute({ ...editingRoute, activeTargetIdx: idx })}
                >
                  <span>🎯 {tgt.config?.name || `Destination ${idx + 1}`}</span>
                  <span style={{ fontSize: '0.75rem', opacity: 0.8 }}>({tgt.protocol?.toUpperCase()})</span>
                  {tgt.config?.enabled === false && <span style={{ color: '#EF4444', fontSize: '10px' }}>(Disabled)</span>}
                </button>
              ))}
              <button className="btn" onClick={addDestination} style={{ borderRadius: '20px', padding: '0.4rem 0.9rem', fontSize: '0.85rem', display: 'flex', alignItems: 'center', gap: '0.25rem', backgroundColor: 'var(--bg)', border: '1px dashed var(--primary)', color: 'var(--primary)' }}>
                <Plus size={14} /> Add Destination
              </button>
            </div>
          </div>

          {/* Active Destination Editor Card */}
          <div className="editor-card">
            {/* Destination Header Controls */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.25rem', borderBottom: '1px solid var(--border)', paddingBottom: '0.75rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '1rem', flex: 1 }}>
                <div style={{ flex: 1, maxWidth: '350px' }}>
                  <label htmlFor={`dest-name-${activeDestIdx}`} style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Destination Name</label>
                  <input
                    id={`dest-name-${activeDestIdx}`}
                    className="form-control"
                    value={activeDest.config?.name || ''}
                    onChange={e => updateActiveDestConfig({ name: e.target.value })}
                    placeholder="e.g. Satu Sehat FHIR, Archive DB"
                    style={{ fontWeight: 600 }}
                  />
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem', marginTop: '1.2rem' }}>
                  <input
                    type="checkbox"
                    id={`dest-enable-${activeDestIdx}`}
                    checked={activeDest.config?.enabled !== false}
                    onChange={e => updateActiveDestConfig({ enabled: e.target.checked })}
                    style={{ width: '18px', height: '18px', cursor: 'pointer' }}
                  />
                  <label htmlFor={`dest-enable-${activeDestIdx}`} style={{ margin: 0, cursor: 'pointer', fontWeight: 500, fontSize: '0.85rem' }}>
                    {activeDest.config?.enabled !== false ? 'Enabled' : 'Disabled'}
                  </label>
                </div>
              </div>

              <div style={{ display: 'flex', gap: '0.75rem', alignItems: 'center' }}>
                {destTest?.state === 'loading' && <span style={{ fontSize: '0.85rem', color: 'var(--text-muted)' }}>Testing... ⏳</span>}
                {destTest?.state === 'success' && <span style={{ fontSize: '0.85rem', color: '#10B981', fontWeight: 600 }}>✅ {destTest.message}</span>}
                {destTest?.state === 'error' && <span style={{ fontSize: '0.85rem', color: '#EF4444', fontWeight: 600 }}>❌ {destTest.message}</span>}
                <button 
                  className="btn btn-test" 
                  onClick={() => testConnection('target', activeDestIdx)} 
                  style={{ padding: '0.45rem 1rem', fontSize: '0.85rem', display: 'flex', alignItems: 'center', gap: '0.4rem', cursor: 'pointer' }}
                >
                  <Play size={14} /> Test Connection
                </button>
                {targets.length > 1 && (
                  <button className="btn btn-danger" onClick={() => removeDestination(activeDestIdx)} title="Delete this destination" style={{ padding: '0.35rem 0.75rem' }}>
                    <Trash2 size={16} />
                  </button>
                )}
              </div>
            </div>

            {/* Reusable Destination Connector Loader Bar */}
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.25rem', padding: '0.6rem 0.9rem', backgroundColor: 'var(--bg)', borderRadius: '8px', border: '1px solid var(--border)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.6rem', flex: 1, maxWidth: '480px' }}>
                <span style={{ fontSize: '0.85rem', fontWeight: 600, color: 'var(--primary)', whiteSpace: 'nowrap' }}>⚡ Saved Connector:</span>
                <select
                  className="form-control"
                  style={{ padding: '0.35rem 0.6rem', fontSize: '0.85rem' }}
                  onChange={(e) => {
                    const conn = savedConnectors.find(c => c.id === e.target.value);
                    if (conn) {
                      updateActiveDest(curr => ({
                        ...curr,
                        protocol: conn.protocol,
                        format: conn.format || curr.format,
                        config: {
                          ...(curr.config || {}),
                          ...(conn.config || {}),
                          name: conn.name || curr.config?.name
                        }
                      }));
                    }
                  }}
                  defaultValue=""
                >
                  <option value="">-- Load from Reusable Connector Profile --</option>
                  {savedConnectors.filter(c => c.role === 'target' || c.role === 'shared').map(c => (
                    <option key={c.id} value={c.id}>{c.name} ({c.protocol?.toUpperCase()})</option>
                  ))}
                </select>
              </div>
              <button
                type="button"
                className="btn btn-outline"
                style={{ padding: '0.35rem 0.75rem', fontSize: '0.8rem' }}
                onClick={() => handleSaveAsConnector('target')}
              >
                💾 Save as Profile
              </button>
            </div>

            {/* Destination Protocol Settings */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(260px, 1fr))', gap: '1rem' }}>
              <div className="form-group">
                <label htmlFor={`dest-protocol-${activeDestIdx}`}>Protocol / Method</label>
                <select
                  id={`dest-protocol-${activeDestIdx}`}
                  className="form-control"
                  value={activeDest.protocol}
                  onChange={e => updateActiveDest(curr => ({ ...curr, protocol: e.target.value }))}
                >
                  <option value="http">HTTP POST</option>
                  <option value="tcp">TCP Client</option>
                  <option value="database">Database Insert / Query</option>
                  <option value="internal-channel">Forward to Another Channel</option>
                </select>
              </div>
              {activeDest.protocol !== 'internal-channel' && (
                <div className="form-group">
                  <label htmlFor={`dest-format-${activeDestIdx}`}>Data Format</label>
                  <select
                    id={`dest-format-${activeDestIdx}`}
                    className="form-control"
                    value={activeDest.format}
                    onChange={e => updateActiveDest(curr => ({ ...curr, format: e.target.value }))}
                  >
                    {(registry.formats.length > 0 ? registry.formats : [
                      { code: 'json', name: 'JSON' },
                      { code: 'xml', name: 'XML' },
                      { code: 'hl7', name: 'HL7 v2' },
                      { code: 'fhir', name: 'FHIR' },
                      { code: 'raw', name: 'Raw String' }
                    ]).map(f => (
                      <option key={f.code} value={f.code}>{f.name}</option>
                    ))}
                  </select>
                </div>
              )}
            </div>

            {activeDest.protocol !== 'internal-channel' && (
              <>
                <div style={{ display: 'grid', gridTemplateColumns: '2fr 120px 2fr', gap: '1rem' }}>
                  <div className="form-group">
                    <label htmlFor={`dest-host-${activeDestIdx}`}>Host / IP Address</label>
                    <input id={`dest-host-${activeDestIdx}`} type="text" className="form-control" placeholder="e.g. https://api-satusehat-stg.dto.kemkes.go.id or 192.168.1.100"
                      value={activeDest.config?.host || ''}
                      onChange={e => updateActiveDestConfig({ host: e.target.value })}
                    />
                  </div>
                  <div className="form-group">
                    <label htmlFor={`dest-port-${activeDestIdx}`}>Port</label>
                    <input id={`dest-port-${activeDestIdx}`} type="text" className="form-control" placeholder="e.g. 8080"
                      value={activeDest.config?.port || ''}
                      onChange={e => updateActiveDestConfig({ port: e.target.value })}
                    />
                  </div>
                  {activeDest.protocol === 'http' && (
                    <div className="form-group">
                      <label htmlFor={`dest-path-${activeDestIdx}`}>Endpoint / Path</label>
                      <input id={`dest-path-${activeDestIdx}`} type="text" className="form-control" placeholder="e.g. /oauth2/v1 or /fhir-r4/v1/Observation"
                        value={activeDest.config?.path || ''}
                        onChange={e => updateActiveDestConfig({ path: e.target.value })}
                      />
                    </div>
                  )}
                </div>

                {activeDest.protocol === 'http' && (
                  <div style={{ display: 'grid', gridTemplateColumns: '220px 1fr', gap: '1rem' }}>
                    <div className="form-group">
                      <label htmlFor={`dest-auth-${activeDestIdx}`}>Authentication</label>
                      <select id={`dest-auth-${activeDestIdx}`} className="form-control"
                        value={activeDest.config?.authType || 'none'}
                        onChange={e => updateActiveDestConfig({ authType: e.target.value })}
                      >
                        <option value="none">None</option>
                        <option value="basic">Basic Auth (User/Password)</option>
                        <option value="bearer">Bearer Token</option>
                      </select>
                    </div>

                    {activeDest.config?.authType === 'bearer' && (
                      <div className="form-group">
                        <label htmlFor={`dest-token-${activeDestIdx}`}>Bearer Token</label>
                        <input id={`dest-token-${activeDestIdx}`} type="password" className="form-control" placeholder="Enter bearer token or {{SATUSEHAT_TOKEN}}..."
                          value={activeDest.config?.token || ''}
                          onChange={e => updateActiveDestConfig({ token: e.target.value })}
                        />
                      </div>
                    )}

                    {activeDest.config?.authType === 'basic' && (
                      <div style={{ display: 'flex', gap: '0.5rem' }}>
                        <div className="form-group" style={{ flex: 1 }}>
                          <label htmlFor={`dest-auth-user-${activeDestIdx}`}>Username</label>
                          <input id={`dest-auth-user-${activeDestIdx}`} type="text" className="form-control"
                            value={activeDest.config?.username || ''}
                            onChange={e => updateActiveDestConfig({ username: e.target.value })}
                          />
                        </div>
                        <div className="form-group" style={{ flex: 1 }}>
                          <label htmlFor={`dest-auth-pass-${activeDestIdx}`}>Password</label>
                          <input id={`dest-auth-pass-${activeDestIdx}`} type="password" className="form-control"
                            value={activeDest.config?.password || ''}
                            onChange={e => updateActiveDestConfig({ password: e.target.value })}
                          />
                        </div>
                      </div>
                    )}
                  </div>
                )}
              </>
            )}

            {activeDest.protocol === 'internal-channel' && (
              <div className="form-group" style={{ marginTop: '0.5rem' }}>
                <label htmlFor={`dest-internal-route-${activeDestIdx}`}>Select Destination Channel</label>
                <select id={`dest-internal-route-${activeDestIdx}`} className="form-control"
                  value={activeDest.config?.route_id || ''}
                  onChange={e => updateActiveDestConfig({ route_id: e.target.value })}
                >
                  <option value="">-- Select Channel --</option>
                  {routes.filter(r => r.id !== editingRoute.id).map(r => (
                    <option key={r.id} value={r.id}>{r.name}</option>
                  ))}
                </select>
                <div style={{ fontSize: '0.75rem', color: 'var(--text-muted)', marginTop: '4px' }}>
                  Messages passing the destination filter will be forwarded into the selected channel's internal queue.
                </div>
              </div>
            )}
          </div>

          {/* Destination Filter Card */}
          <div className="filter-card" style={{ borderLeftColor: '#3B82F6' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: '0.5rem' }}>
                <Filter size={18} color="#3B82F6" />
                <h4 style={{ margin: 0, fontSize: '1rem', color: '#3B82F6', fontWeight: 600 }}>
                  Destination Filter: {activeDest.config?.name || `Destination ${activeDestIdx + 1}`}
                </h4>
                <span className="filter-badge" style={{ backgroundColor: 'rgba(59, 130, 246, 0.1)', color: '#3B82F6' }}>
                  Routing Rule
                </span>
              </div>
              <div style={{ fontSize: '0.8rem', color: 'var(--text-muted)' }}>
                Only messages returning <code>true</code> will be forwarded to this target.
              </div>
            </div>
            <textarea
              className="form-control editor-textarea"
              style={{ fontSize: '12px', minHeight: '100px', borderLeft: '3px solid #3B82F6', backgroundColor: '#0d1117', color: '#58a6ff' }}
              placeholder="// JavaScript filter condition for this destination. Examples:&#10;return msg.department === 'LAB';&#10;// or:&#10;return JSON.parse(input).send_to_satusehat === true;"
              value={activeDest.config?.filter_script || ''}
              onChange={e => updateActiveDestConfig({ filter_script: e.target.value })}
            />
          </div>

          {/* Destination Transformer & Reference Template Card */}
          <div className="editor-card">
            <h3 style={{ margin: '0 0 0.5rem 0', color: 'var(--primary)', fontSize: '1.1rem' }}>
              🔄 Destination Transformer & Mapping
            </h3>
            <p style={{ fontSize: '0.875rem', color: 'var(--text-muted)', marginBottom: '1rem' }}>
              Transform data for this destination. Available functions: <code>http.request()</code>, <code>cache.get/set()</code>, <code>getGlobalVar("KEY")</code>, and dynamic routing via <code>__dispatch</code>.
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1.5rem' }}>
              <div className="form-group" style={{ margin: 0 }}>
                <label htmlFor={`dest-trans-script-${activeDestIdx}`} style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>Transformer Script (JavaScript)</span>
                  <span style={{ fontSize: '10px', color: '#818CF8' }}>Mapping & Dynamic Dispatch</span>
                </label>
                <textarea id={`dest-trans-script-${activeDestIdx}`} className="form-control editor-textarea"
                  style={{ fontSize: '12px', minHeight: '280px', borderLeft: '2px solid #818CF8' }}
                  placeholder="// Format payload, or return dynamic dispatch metadata:&#10;// return JSON.stringify({&#10;//   __dispatch: { url: targetUrl, method: 'POST', headers: { Authorization: 'Bearer ' + token } },&#10;//   payload: finalData&#10;// });"
                  value={activeDest.config?.transformer_script || ''}
                  onChange={e => updateActiveDestConfig({ transformer_script: e.target.value })}
                />
              </div>
              <div className="form-group" style={{ margin: 0 }}>
                <label htmlFor={`dest-out-template-${activeDestIdx}`} style={{ display: 'flex', justifyContent: 'space-between' }}>
                  <span>Target Reference (Expected Payload)</span>
                  <span style={{ fontSize: '10px', color: '#818CF8' }}>Outbound Reference</span>
                </label>
                <textarea id={`dest-out-template-${activeDestIdx}`} className="form-control editor-textarea"
                  style={{ fontSize: '12px', minHeight: '280px' }}
                  placeholder="Paste the target system's expected JSON / FHIR / XML format here for reference..."
                  value={activeDest.config?.outbound_template || ''}
                  onChange={e => updateActiveDestConfig({ outbound_template: e.target.value })}
                />
              </div>
            </div>
          </div>

          {/* Destination Scoped Variables Card */}
          <VariablesEditor
            title={'🔑 Destination Variables (' + (activeDest.config?.name || 'Dest ' + (activeDestIdx + 1)) + ')'}
            description={<>Define variables used by this destination (e.g. <code>CLIENT_ID</code>, <code>SECRET_KEY</code>, <code>BEARER_TOKEN</code>).</>}
            variables={activeDest.config?.variables || {}}
            onChange={next => updateActiveDestConfig({ variables: next })}
            addLabel="+ Add Destination Variable"
            varPrefix={`dest_${activeDestIdx}`}
            visibleVars={visibleVars}
            onToggleVisibility={k => setVisibleVars(prev => ({ ...prev, [k]: !prev[k] }))}
          />
        </div>
      )}
    </div>
  );
}
