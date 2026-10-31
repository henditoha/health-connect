import React, { useState, useEffect } from 'react';
import { Plus, Play, Trash2, Edit2, ShieldCheck, KeyRound, Network, RefreshCw, Check, X } from './icons';

export default function ConnectorsView({ apiBase }) {
  const [activeSubTab, setActiveSubTab] = useState('connectors'); // 'connectors' | 'registry'
  const [connectors, setConnectors] = useState([]);
  const [registry, setRegistry] = useState({ protocols: [], database_drivers: [], formats: [] });
  const [loading, setLoading] = useState(false);
  const [roleFilter, setRoleFilter] = useState('all');
  const [testResults, setTestResults] = useState({});

  // Modal State
  const [editingConnector, setEditingConnector] = useState(null); // null = closed, {} = new, obj = edit
  const [editingDriver, setEditingDriver] = useState(null); // null = closed, {} = new

  useEffect(() => {
    loadData();
  }, []);

  const loadData = async () => {
    setLoading(true);
    try {
      const [resConn, resReg] = await Promise.all([
        fetch(`${apiBase}/connectors`),
        fetch(`${apiBase}/connector-registry`)
      ]);
      const dataConn = await resConn.json();
      const dataReg = await resReg.json();

      if (Array.isArray(dataConn)) setConnectors(dataConn);
      if (dataReg && dataReg.protocols) setRegistry(dataReg);
    } catch (err) {
      console.error('Failed to load connector data:', err);
    } finally {
      setLoading(false);
    }
  };

  const handleTestConnection = async (id) => {
    setTestResults(prev => ({ ...prev, [id]: { loading: true } }));
    try {
      const res = await fetch(`${apiBase}/connectors/${id}/test`, { method: 'POST' });
      const data = await res.json();
      if (res.ok && data.success) {
        setTestResults(prev => ({ ...prev, [id]: { loading: false, success: true, message: data.message } }));
      } else {
        setTestResults(prev => ({ ...prev, [id]: { loading: false, success: false, message: data.error || data.message || 'Connection failed' } }));
      }
    } catch (err) {
      setTestResults(prev => ({ ...prev, [id]: { loading: false, success: false, message: err.message } }));
    }
  };

  const handleDeleteConnector = async (id, name) => {
    if (!window.confirm(`Delete connector profile "${name}"?`)) return;
    try {
      const res = await fetch(`${apiBase}/connectors/${id}`, { method: 'DELETE' });
      if (res.ok) {
        setConnectors(prev => prev.filter(c => c.id !== id));
      } else {
        const err = await res.json();
        alert('Failed: ' + (err.error || 'Unknown error'));
      }
    } catch (err) {
      alert('Network error: ' + err.message);
    }
  };

  const handleSaveConnector = async (e) => {
    e.preventDefault();
    const isEdit = !!editingConnector.id;
    const url = isEdit ? `${apiBase}/connectors/${editingConnector.id}` : `${apiBase}/connectors`;
    const method = isEdit ? 'PUT' : 'POST';

    try {
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(editingConnector)
      });
      if (res.ok) {
        setEditingConnector(null);
        loadData();
      } else {
        const err = await res.json();
        alert('Save failed: ' + (err.error || 'Unknown error'));
      }
    } catch (err) {
      alert('Network error: ' + err.message);
    }
  };

  const handleSaveDriver = async (e) => {
    e.preventDefault();
    try {
      const res = await fetch(`${apiBase}/connector-registry`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          category: 'database_driver',
          code: editingDriver.code,
          name: editingDriver.name,
          description: editingDriver.description,
          sample_config: editingDriver.sample_config || ''
        })
      });
      if (res.ok) {
        setEditingDriver(null);
        loadData();
      } else {
        const err = await res.json();
        alert('Failed: ' + (err.error || 'Unknown error'));
      }
    } catch (err) {
      alert('Network error: ' + err.message);
    }
  };

  const handleDeleteDriver = async (id, name) => {
    if (!window.confirm(`Remove driver "${name}" from registry?`)) return;
    try {
      const res = await fetch(`${apiBase}/connector-registry/${id}`, { method: 'DELETE' });
      if (res.ok) {
        loadData();
      }
    } catch (err) {
      alert('Network error: ' + err.message);
    }
  };

  const filteredConnectors = connectors.filter(c => {
    if (roleFilter === 'all') return true;
    return c.role === roleFilter || c.role === 'shared';
  });

  return (
    <div className="tab-pane active" style={{ padding: '1.5rem', maxWidth: '1400px', margin: '0 auto' }}>
      {/* Top Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '1.5rem', flexWrap: 'wrap', gap: '1rem' }}>
        <div>
          <h2 style={{ margin: 0, fontSize: '1.5rem', fontWeight: 700, display: 'flex', alignItems: 'center', gap: '0.6rem', color: 'var(--primary)' }}>
            <Network size={24} /> Connectors & Protocols Management
          </h2>
          <p style={{ margin: '0.35rem 0 0', color: 'var(--text-muted)', fontSize: '0.9rem' }}>
            Kelola profil koneksi terpusat (Reusable Connectors) dan registri driver database dinamis tanpa harus mengubah kode.
          </p>
        </div>

        <div style={{ display: 'flex', gap: '0.75rem' }}>
          <button
            className="btn btn-outline"
            onClick={loadData}
            style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 0.9rem' }}
          >
            <RefreshCw size={14} className={loading ? 'spin' : ''} /> Refresh
          </button>
          {activeSubTab === 'connectors' ? (
            <button
              className="btn btn-primary"
              onClick={() => setEditingConnector({
                name: '',
                role: 'shared',
                protocol: 'http',
                format: 'json',
                config: {},
                description: ''
              })}
              style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 1rem', fontWeight: 600 }}
            >
              <Plus size={16} /> New Connector Profile
            </button>
          ) : (
            <button
              className="btn btn-primary"
              onClick={() => setEditingDriver({ code: '', name: '', description: '', sample_config: '' })}
              style={{ display: 'flex', alignItems: 'center', gap: '0.4rem', padding: '0.5rem 1rem', fontWeight: 600 }}
            >
              <Plus size={16} /> Register New Driver
            </button>
          )}
        </div>
      </div>

      {/* Sub Tabs */}
      <div style={{ display: 'flex', gap: '0.5rem', borderBottom: '1px solid var(--border)', marginBottom: '1.5rem' }}>
        <button
          className={`btn ${activeSubTab === 'connectors' ? 'btn-primary' : 'btn-outline'}`}
          onClick={() => setActiveSubTab('connectors')}
          style={{ borderRadius: '8px 8px 0 0', borderBottom: 'none', padding: '0.6rem 1.2rem', fontWeight: 600 }}
        >
          🔌 Reusable Connectors ({connectors.length})
        </button>
        <button
          className={`btn ${activeSubTab === 'registry' ? 'btn-primary' : 'btn-outline'}`}
          onClick={() => setActiveSubTab('registry')}
          style={{ borderRadius: '8px 8px 0 0', borderBottom: 'none', padding: '0.6rem 1.2rem', fontWeight: 600 }}
        >
          🗂️ Driver & Protocol Registry ({registry.database_drivers.length} Drivers)
        </button>
      </div>

      {/* SUB-TAB 1: REUSABLE CONNECTORS */}
      {activeSubTab === 'connectors' && (
        <div>
          {/* Filter Pills */}
          <div style={{ display: 'flex', gap: '0.5rem', marginBottom: '1rem', alignItems: 'center' }}>
            <span style={{ fontSize: '0.8rem', color: 'var(--text-muted)', fontWeight: 600 }}>Role Filter:</span>
            {['all', 'source', 'target', 'shared'].map(r => (
              <button
                key={r}
                className={`tag ${roleFilter === r ? 'active' : ''}`}
                onClick={() => setRoleFilter(r)}
                style={{
                  cursor: 'pointer',
                  padding: '0.25rem 0.75rem',
                  borderRadius: '20px',
                  backgroundColor: roleFilter === r ? 'var(--primary)' : 'var(--bg-card)',
                  color: roleFilter === r ? '#fff' : 'var(--text)',
                  border: '1px solid var(--border)',
                  textTransform: 'capitalize'
                }}
              >
                {r}
              </button>
            ))}
          </div>

          {filteredConnectors.length === 0 ? (
            <div style={{ padding: '3rem', textAlign: 'center', backgroundColor: 'var(--bg-card)', borderRadius: '12px', border: '1px dashed var(--border)' }}>
              <Network size={36} color="var(--text-muted)" style={{ marginBottom: '0.5rem' }} />
              <h4 style={{ margin: 0, color: 'var(--text)' }}>No Connectors Found</h4>
              <p style={{ color: 'var(--text-muted)', fontSize: '0.85rem', marginTop: '0.25rem' }}>
                Tambahkan profil konektor baru untuk dipakai ulang di berbagai channel tanpa mengetik ulang DSN/IP.
              </p>
            </div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(380px, 1fr))', gap: '1rem' }}>
              {filteredConnectors.map(c => {
                const test = testResults[c.id];
                return (
                  <div
                    key={c.id}
                    style={{
                      backgroundColor: 'var(--bg-card)',
                      borderRadius: '12px',
                      border: '1px solid var(--border)',
                      padding: '1.25rem',
                      display: 'flex',
                      flexDirection: 'column',
                      justifyContent: 'space-between',
                      boxShadow: '0 2px 4px rgba(0,0,0,0.03)'
                    }}
                  >
                    <div>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '0.5rem' }}>
                        <h4 style={{ margin: 0, fontSize: '1.05rem', fontWeight: 700, color: 'var(--primary)' }}>
                          {c.name}
                        </h4>
                        <div style={{ display: 'flex', gap: '0.3rem' }}>
                          <span className="tag" style={{ backgroundColor: 'var(--primary-light)', color: 'var(--primary)', fontWeight: 600, fontSize: '0.7rem' }}>
                            {c.protocol?.toUpperCase()}
                          </span>
                          <span className="tag" style={{ backgroundColor: '#f3f4f6', color: '#4b5563', fontSize: '0.7rem' }}>
                            {c.role?.toUpperCase()}
                          </span>
                        </div>
                      </div>

                      <p style={{ margin: '0 0 0.75rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                        {c.description || 'Tidak ada deskripsi'}
                      </p>

                      <div style={{ backgroundColor: 'var(--bg)', padding: '0.6rem 0.8rem', borderRadius: '6px', fontSize: '0.8rem', fontFamily: 'monospace', marginBottom: '0.75rem', wordBreak: 'break-all', border: '1px solid var(--border)' }}>
                        {c.protocol === 'database' && (
                          <div>
                            <div><strong>Driver:</strong> {c.config?.driver || 'mysql'}</div>
                            <div><strong>DSN:</strong> {c.config?.dsn || '(not set)'}</div>
                          </div>
                        )}
                        {c.protocol === 'http' && (
                          <div>
                            <div><strong>Target URL:</strong> {c.config?.url || c.config?.host || '/ingress'}</div>
                            {c.config?.authType && <div><strong>Auth:</strong> {c.config.authType}</div>}
                          </div>
                        )}
                        {c.protocol === 'tcp' && (
                          <div><strong>Listen Port:</strong> {c.config?.port || '(not set)'}</div>
                        )}
                        {c.protocol === 'http-poll' && (
                          <div><strong>Poll URL:</strong> {c.config?.url || '(not set)'}</div>
                        )}
                      </div>

                      {test && (
                        <div style={{
                          padding: '0.4rem 0.6rem',
                          borderRadius: '6px',
                          fontSize: '0.75rem',
                          marginBottom: '0.75rem',
                          backgroundColor: test.loading ? 'rgba(59, 130, 246, 0.1)' : test.success ? 'rgba(16, 185, 129, 0.1)' : 'rgba(239, 68, 68, 0.1)',
                          color: test.loading ? '#3b82f6' : test.success ? '#10b981' : '#ef4444',
                          fontWeight: 600
                        }}>
                          {test.loading ? 'Menguji koneksi... ⏳' : test.success ? `✅ ${test.message}` : `❌ ${test.message}`}
                        </div>
                      )}
                    </div>

                    <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', paddingTop: '0.75rem', borderTop: '1px solid var(--border)' }}>
                      <button
                        className="btn btn-outline"
                        onClick={() => handleTestConnection(c.id)}
                        disabled={test?.loading}
                        style={{ padding: '0.35rem 0.8rem', fontSize: '0.8rem', display: 'flex', alignItems: 'center', gap: '0.3rem' }}
                      >
                        <Play size={12} /> Test
                      </button>

                      <div style={{ display: 'flex', gap: '0.4rem' }}>
                        <button
                          className="btn btn-outline"
                          onClick={() => setEditingConnector(c)}
                          style={{ padding: '0.35rem 0.6rem', fontSize: '0.8rem' }}
                          title="Edit Connector"
                        >
                          <Edit2 size={13} />
                        </button>
                        <button
                          className="btn btn-danger"
                          onClick={() => handleDeleteConnector(c.id, c.name)}
                          style={{ padding: '0.35rem 0.6rem', fontSize: '0.8rem' }}
                          title="Delete Connector"
                        >
                          <Trash2 size={13} />
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          )}
        </div>
      )}

      {/* SUB-TAB 2: DRIVER & PROTOCOL REGISTRY */}
      {activeSubTab === 'registry' && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2rem' }}>
          {/* Database Drivers */}
          <div style={{ backgroundColor: 'var(--bg-card)', padding: '1.25rem', borderRadius: '12px', border: '1px solid var(--border)' }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem' }}>
              <div>
                <h3 style={{ margin: 0, fontSize: '1.15rem', fontWeight: 700, color: 'var(--primary)' }}>
                  Database Drivers Registry
                </h3>
                <p style={{ margin: '0.2rem 0 0', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
                  Daftar driver basis data yang terdaftar dalam engine Health-Connect. Driver baru bisa ditambahkan tanpa compile ulang frontend.
                </p>
              </div>
            </div>

            <div style={{ overflowX: 'auto' }}>
              <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.85rem' }}>
                <thead>
                  <tr style={{ textAlign: 'left', borderBottom: '1px solid var(--border)', color: 'var(--text-muted)' }}>
                    <th style={{ padding: '0.6rem' }}>Driver Code</th>
                    <th style={{ padding: '0.6rem' }}>Display Name</th>
                    <th style={{ padding: '0.6rem' }}>Deskripsi</th>
                    <th style={{ padding: '0.6rem' }}>Sample DSN</th>
                    <th style={{ padding: '0.6rem', textAlign: 'right' }}>Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {registry.database_drivers.map(d => (
                    <tr key={d.id} style={{ borderBottom: '1px solid var(--border)' }}>
                      <td style={{ padding: '0.6rem', fontWeight: 600, fontFamily: 'monospace' }}>
                        <span className="tag" style={{ backgroundColor: 'var(--bg)', border: '1px solid var(--border)' }}>
                          {d.code}
                        </span>
                      </td>
                      <td style={{ padding: '0.6rem', fontWeight: 600 }}>{d.name}</td>
                      <td style={{ padding: '0.6rem', color: 'var(--text-muted)' }}>{d.description}</td>
                      <td style={{ padding: '0.6rem', fontFamily: 'monospace', fontSize: '0.75rem', color: 'var(--text-muted)' }}>
                        {d.sample_config ? JSON.parse(d.sample_config)?.dsn : '-'}
                      </td>
                      <td style={{ padding: '0.6rem', textAlign: 'right' }}>
                        <button
                          className="btn btn-danger"
                          onClick={() => handleDeleteDriver(d.id, d.name)}
                          style={{ padding: '0.2rem 0.5rem', fontSize: '0.75rem' }}
                          title="Hapus Driver dari Registry"
                        >
                          <Trash2 size={12} />
                        </button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>

          {/* Protocols */}
          <div style={{ backgroundColor: 'var(--bg-card)', padding: '1.25rem', borderRadius: '12px', border: '1px solid var(--border)' }}>
            <h3 style={{ margin: '0 0 0.4rem', fontSize: '1.15rem', fontWeight: 700, color: 'var(--primary)' }}>
              Supported Protocols
            </h3>
            <p style={{ margin: '0 0 1rem', fontSize: '0.85rem', color: 'var(--text-muted)' }}>
              Protokol integrasi yang didukung secara modular oleh Health-Connect Engine.
            </p>
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(240px, 1fr))', gap: '0.75rem' }}>
              {registry.protocols.map(p => (
                <div key={p.id} style={{ padding: '0.75rem', borderRadius: '8px', border: '1px solid var(--border)', backgroundColor: 'var(--bg)' }}>
                  <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
                    <span style={{ fontWeight: 700, fontSize: '0.9rem' }}>{p.name}</span>
                    <span className="tag" style={{ fontSize: '0.7rem' }}>{p.code}</span>
                  </div>
                  <p style={{ margin: '0.3rem 0 0', fontSize: '0.75rem', color: 'var(--text-muted)' }}>{p.description}</p>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}

      {/* MODAL: ADD / EDIT CONNECTOR PROFILE */}
      {editingConnector && (
        <div style={{
          position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex', justifyContent: 'center', alignItems: 'center', zIndex: 9999, padding: '1rem'
        }}>
          <div style={{
            backgroundColor: 'var(--bg-card)', borderRadius: '12px', border: '1px solid var(--border)',
            width: '100%', maxWidth: '650px', maxHeight: '90vh', overflowY: 'auto', padding: '1.5rem', boxShadow: '0 20px 25px -5px rgba(0,0,0,0.1)'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1.25rem', borderBottom: '1px solid var(--border)', paddingBottom: '0.75rem' }}>
              <h3 style={{ margin: 0, fontSize: '1.2rem', fontWeight: 700, color: 'var(--primary)' }}>
                {editingConnector.id ? 'Edit Connector Profile' : 'New Reusable Connector Profile'}
              </h3>
              <button className="btn btn-outline" onClick={() => setEditingConnector(null)} style={{ padding: '0.2rem 0.5rem' }}>
                <X size={16} />
              </button>
            </div>

            <form onSubmit={handleSaveConnector}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
                <div className="form-group">
                  <label>Connector Name *</label>
                  <input
                    type="text"
                    required
                    className="form-control"
                    placeholder="e.g. SIMRS IRIS Database, SATUSEHAT Gateway"
                    value={editingConnector.name || ''}
                    onChange={e => setEditingConnector({ ...editingConnector, name: e.target.value })}
                  />
                </div>
                <div className="form-group">
                  <label>Role</label>
                  <select
                    className="form-control"
                    value={editingConnector.role || 'shared'}
                    onChange={e => setEditingConnector({ ...editingConnector, role: e.target.value })}
                  >
                    <option value="shared">Shared (Source & Target)</option>
                    <option value="source">Source Only</option>
                    <option value="target">Target Only</option>
                  </select>
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '1rem' }}>
                <div className="form-group">
                  <label>Protocol *</label>
                  <select
                    className="form-control"
                    value={editingConnector.protocol || 'http'}
                    onChange={e => setEditingConnector({ ...editingConnector, protocol: e.target.value })}
                  >
                    {registry.protocols.map(p => (
                      <option key={p.code} value={p.code}>{p.name}</option>
                    ))}
                  </select>
                </div>
                <div className="form-group">
                  <label>Data Format</label>
                  <select
                    className="form-control"
                    value={editingConnector.format || 'json'}
                    onChange={e => setEditingConnector({ ...editingConnector, format: e.target.value })}
                  >
                    {registry.formats.map(f => (
                      <option key={f.code} value={f.code}>{f.name}</option>
                    ))}
                  </select>
                </div>
              </div>

              {/* Dynamic Protocol Fields */}
              <div style={{ padding: '1rem', backgroundColor: 'var(--bg)', borderRadius: '8px', border: '1px solid var(--border)', marginBottom: '1rem' }}>
                <h4 style={{ margin: '0 0 0.75rem', fontSize: '0.9rem', color: 'var(--primary)', fontWeight: 600 }}>
                  Configuration: {editingConnector.protocol?.toUpperCase()}
                </h4>

                {editingConnector.protocol === 'database' && (
                  <div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '0.75rem' }}>
                      <div className="form-group">
                        <label>Database Driver</label>
                        <select
                          className="form-control"
                          value={editingConnector.config?.driver || 'mysql'}
                          onChange={e => setEditingConnector({
                            ...editingConnector,
                            config: { ...(editingConnector.config || {}), driver: e.target.value }
                          })}
                        >
                          {registry.database_drivers.map(d => (
                            <option key={d.code} value={d.code}>{d.name}</option>
                          ))}
                        </select>
                      </div>
                      <div className="form-group">
                        <label>Poll Interval (Seconds)</label>
                        <input
                          type="number"
                          className="form-control"
                          placeholder="60"
                          value={editingConnector.config?.pollInterval || '60'}
                          onChange={e => setEditingConnector({
                            ...editingConnector,
                            config: { ...(editingConnector.config || {}), pollInterval: e.target.value }
                          })}
                        />
                      </div>
                    </div>
                    <div className="form-group" style={{ marginBottom: '0.75rem' }}>
                      <label>Connection String (DSN)</label>
                      <input
                        type="text"
                        className="form-control"
                        placeholder="e.g. DSN=SIMRS_IRIS;UID=user;PWD=pass; or user:pass@tcp(host:3306)/db"
                        value={editingConnector.config?.dsn || ''}
                        onChange={e => setEditingConnector({
                          ...editingConnector,
                          config: { ...(editingConnector.config || {}), dsn: e.target.value }
                        })}
                      />
                    </div>
                    <div className="form-group">
                      <label>Default Poller Query (Optional)</label>
                      <textarea
                        className="form-control"
                        rows="2"
                        placeholder="SELECT id, norm, nama FROM table WHERE status='new'"
                        value={editingConnector.config?.query || ''}
                        onChange={e => setEditingConnector({
                          ...editingConnector,
                          config: { ...(editingConnector.config || {}), query: e.target.value }
                        })}
                      />
                    </div>
                  </div>
                )}

                {editingConnector.protocol === 'http' && (
                  <div>
                    <div className="form-group" style={{ marginBottom: '0.75rem' }}>
                      <label>Host / Base URL</label>
                      <input
                        type="text"
                        className="form-control"
                        placeholder="e.g. https://api-satusehat-stg.kemkes.go.id"
                        value={editingConnector.config?.host || editingConnector.config?.url || ''}
                        onChange={e => setEditingConnector({
                          ...editingConnector,
                          config: { ...(editingConnector.config || {}), host: e.target.value, url: e.target.value }
                        })}
                      />
                    </div>
                    <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '1rem', marginBottom: '0.75rem' }}>
                      <div className="form-group">
                        <label>Endpoint / Path</label>
                        <input
                          type="text"
                          className="form-control"
                          placeholder="/fhir-r4/v1"
                          value={editingConnector.config?.path || ''}
                          onChange={e => setEditingConnector({
                            ...editingConnector,
                            config: { ...(editingConnector.config || {}), path: e.target.value }
                          })}
                        />
                      </div>
                      <div className="form-group">
                        <label>Authentication Type</label>
                        <select
                          className="form-control"
                          value={editingConnector.config?.authType || 'none'}
                          onChange={e => setEditingConnector({
                            ...editingConnector,
                            config: { ...(editingConnector.config || {}), authType: e.target.value }
                          })}
                        >
                          <option value="none">None</option>
                          <option value="bearer">Bearer Token</option>
                          <option value="basic">Basic Auth</option>
                        </select>
                      </div>
                    </div>
                    {editingConnector.config?.authType === 'bearer' && (
                      <div className="form-group">
                        <label>Bearer Token or Variable</label>
                        <input
                          type="text"
                          className="form-control"
                          placeholder="{{SATUSEHAT_TOKEN}} or token value"
                          value={editingConnector.config?.token || ''}
                          onChange={e => setEditingConnector({
                            ...editingConnector,
                            config: { ...(editingConnector.config || {}), token: e.target.value }
                          })}
                        />
                      </div>
                    )}
                  </div>
                )}

                {editingConnector.protocol === 'tcp' && (
                  <div className="form-group">
                    <label>TCP Socket Port</label>
                    <input
                      type="text"
                      className="form-control"
                      placeholder="e.g. 9000"
                      value={editingConnector.config?.port || ''}
                      onChange={e => setEditingConnector({
                        ...editingConnector,
                        config: { ...(editingConnector.config || {}), port: e.target.value }
                      })}
                    />
                  </div>
                )}
              </div>

              <div className="form-group" style={{ marginBottom: '1.25rem' }}>
                <label>Description / Notes</label>
                <textarea
                  className="form-control"
                  rows="2"
                  placeholder="Keterangan koneksi ini (misal: SIMRS Rawat Jalan Produksi)"
                  value={editingConnector.description || ''}
                  onChange={e => setEditingConnector({ ...editingConnector, description: e.target.value })}
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem' }}>
                <button type="button" className="btn btn-outline" onClick={() => setEditingConnector(null)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" style={{ fontWeight: 600 }}>
                  Save Connector Profile
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* MODAL: REGISTER NEW DATABASE DRIVER */}
      {editingDriver && (
        <div style={{
          position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(0,0,0,0.5)', display: 'flex', justifyContent: 'center', alignItems: 'center', zIndex: 9999, padding: '1rem'
        }}>
          <div style={{
            backgroundColor: 'var(--bg-card)', borderRadius: '12px', border: '1px solid var(--border)',
            width: '100%', maxWidth: '520px', padding: '1.5rem', boxShadow: '0 20px 25px -5px rgba(0,0,0,0.1)'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '1rem', borderBottom: '1px solid var(--border)', paddingBottom: '0.75rem' }}>
              <h3 style={{ margin: 0, fontSize: '1.15rem', fontWeight: 700, color: 'var(--primary)' }}>
                Register New Database Driver
              </h3>
              <button className="btn btn-outline" onClick={() => setEditingDriver(null)} style={{ padding: '0.2rem 0.5rem' }}>
                <X size={16} />
              </button>
            </div>

            <form onSubmit={handleSaveDriver}>
              <div className="form-group" style={{ marginBottom: '0.75rem' }}>
                <label>Driver Code *</label>
                <input
                  type="text"
                  required
                  className="form-control"
                  placeholder="e.g. oracle, sybase, clickhouse, firebird"
                  value={editingDriver.code || ''}
                  onChange={e => setEditingDriver({ ...editingDriver, code: e.target.value.toLowerCase().trim() })}
                />
                <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>Kode unik driver (huruf kecil tanpa spasi)</span>
              </div>

              <div className="form-group" style={{ marginBottom: '0.75rem' }}>
                <label>Display Name *</label>
                <input
                  type="text"
                  required
                  className="form-control"
                  placeholder="e.g. Oracle Database (via ODBC)"
                  value={editingDriver.name || ''}
                  onChange={e => setEditingDriver({ ...editingDriver, name: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ marginBottom: '0.75rem' }}>
                <label>Description</label>
                <input
                  type="text"
                  className="form-control"
                  placeholder="e.g. Driver koneksi untuk basis data Oracle 19c"
                  value={editingDriver.description || ''}
                  onChange={e => setEditingDriver({ ...editingDriver, description: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ marginBottom: '1.25rem' }}>
                <label>Sample DSN Connection String</label>
                <input
                  type="text"
                  className="form-control"
                  placeholder='DSN=ORACLE_DB;UID=user;PWD=pass;'
                  value={editingDriver.sample_config || ''}
                  onChange={e => setEditingDriver({ ...editingDriver, sample_config: JSON.stringify({ dsn: e.target.value }) })}
                />
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '0.75rem' }}>
                <button type="button" className="btn btn-outline" onClick={() => setEditingDriver(null)}>
                  Cancel
                </button>
                <button type="submit" className="btn btn-primary" style={{ fontWeight: 600 }}>
                  Register Driver
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
