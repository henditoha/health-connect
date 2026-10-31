import React, { useState, useEffect } from 'react';
import { 
  Scan, Server, Play, Plus, RefreshCw, Trash2, Edit2, 
  Check, X, Activity, Filter, Eye, ChevronDown
} from './icons';

export default function DicomWorklistView() {
  const [items, setItems] = useState([]);
  const [stats, setStats] = useState(null);
  const [loading, setLoading] = useState(true);
  const [dateFilter, setDateFilter] = useState('');
  const [modalityFilter, setModalityFilter] = useState('ALL');
  const [statusFilter, setStatusFilter] = useState('ALL');
  const [search, setSearch] = useState('');
  
  const [modalOpen, setModalOpen] = useState(false);
  const [detailModalItem, setDetailModalItem] = useState(null);
  const [serverSettingsOpen, setServerSettingsOpen] = useState(false);

  // Form State
  const [formData, setFormData] = useState({
    id: '',
    accession_number: '',
    patient_id: '',
    patient_name: '',
    birth_date: '',
    gender: 'M',
    modality: 'DX',
    procedure_desc: '',
    scheduled_date: new Date().toISOString().slice(0, 10).replace(/-/g, ''),
    scheduled_time: '090000',
    station_ae_title: '*',
    physician_name: 'dr. Spesialis Radiologi, Sp.Rad'
  });

  // Server Control State
  const [serverPort, setServerPort] = useState(11112);
  const [serverAET, setServerAET] = useState('HEALTH_CONNECT');

  useEffect(() => {
    fetchWorklist();
    fetchStats();
    const interval = setInterval(() => {
      fetchStats();
    }, 5000);
    return () => clearInterval(interval);
  }, [dateFilter, modalityFilter, statusFilter, search]);

  const fetchWorklist = async () => {
    try {
      setLoading(true);
      const params = new URLSearchParams();
      if (dateFilter) params.append('date', dateFilter);
      if (modalityFilter && modalityFilter !== 'ALL') params.append('modality', modalityFilter);
      if (statusFilter && statusFilter !== 'ALL') params.append('status', statusFilter);
      if (search) params.append('search', search);

      const res = await fetch(`/api/dicom-worklist?${params.toString()}`);
      if (res.ok) {
        const data = await res.json();
        setItems(data.items || []);
      }
    } catch (err) {
      console.error('Failed to fetch worklist:', err);
    } finally {
      setLoading(false);
    }
  };

  const fetchStats = async () => {
    try {
      const res = await fetch('/api/dicom-server/status');
      if (res.ok) {
        const data = await res.json();
        setStats(data);
        if (data.port) setServerPort(data.port);
        if (data.ae_title) setServerAET(data.ae_title);
      }
    } catch (err) {
      console.error('Failed to fetch DICOM server stats:', err);
    }
  };

  const handleControlServer = async (action) => {
    try {
      const res = await fetch('/api/dicom-server/control', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          action,
          port: Number(serverPort),
          ae_title: serverAET
        })
      });
      if (res.ok) {
        fetchStats();
        setServerSettingsOpen(false);
      }
    } catch (err) {
      console.error('Failed to control DICOM server:', err);
    }
  };

  const handleSaveOrder = async (e) => {
    e.preventDefault();
    try {
      const url = formData.id ? `/api/dicom-worklist/${formData.id}` : '/api/dicom-worklist';
      const method = formData.id ? 'PUT' : 'POST';

      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(formData)
      });

      if (res.ok) {
        setModalOpen(false);
        fetchWorklist();
        resetForm();
      }
    } catch (err) {
      console.error('Failed to save order:', err);
    }
  };

  const handleDelete = async (id) => {
    if (!window.confirm('Hapus order pemeriksaan ini dari antrean worklist?')) return;
    try {
      const res = await fetch(`/api/dicom-worklist/${id}`, { method: 'DELETE' });
      if (res.ok) {
        fetchWorklist();
      }
    } catch (err) {
      console.error('Failed to delete worklist item:', err);
    }
  };

  const handleUpdateStatus = async (id, newStatus) => {
    try {
      const res = await fetch(`/api/dicom-worklist/${id}/status`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ status: newStatus })
      });
      if (res.ok) {
        fetchWorklist();
      }
    } catch (err) {
      console.error('Failed to update status:', err);
    }
  };

  const openEditModal = (item) => {
    setFormData({
      id: item.id,
      accession_number: item.accession_number,
      patient_id: item.patient_id,
      patient_name: item.patient_name,
      birth_date: item.birth_date,
      gender: item.gender,
      modality: item.modality,
      procedure_desc: item.procedure_desc,
      scheduled_date: item.scheduled_date,
      scheduled_time: item.scheduled_time,
      station_ae_title: item.station_ae_title,
      physician_name: item.physician_name
    });
    setModalOpen(true);
  };

  const resetForm = () => {
    const today = new Date().toISOString().slice(0, 10).replace(/-/g, '');
    const nowTime = new Date().toTimeString().slice(0, 8).replace(/:/g, '');
    setFormData({
      id: '',
      accession_number: 'ACC' + Date.now().toString().slice(-8),
      patient_id: 'RM' + Math.floor(100000 + Math.random() * 900000),
      patient_name: '',
      birth_date: '19900101',
      gender: 'M',
      modality: 'DX',
      procedure_desc: 'Rontgen Thorax PA/AP Dewasa',
      scheduled_date: today,
      scheduled_time: nowTime,
      station_ae_title: '*',
      physician_name: 'dr. Spesialis Radiologi, Sp.Rad'
    });
  };

  return (
    <div style={{ padding: '24px', maxWidth: '1400px', margin: '0 auto', color: '#1e293b' }}>
      {/* Top Header */}
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'flex-start', marginBottom: '24px' }}>
        <div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '10px' }}>
            <div style={{ padding: '10px', background: 'linear-gradient(135deg, #0284c7, #0369a1)', borderRadius: '12px', color: '#fff' }}>
              <Scan size={24} />
            </div>
            <div>
              <h1 style={{ fontSize: '22px', fontWeight: '700', margin: 0, color: '#0f172a' }}>
                DICOM Modality Worklist (MWL) Broker
              </h1>
              <p style={{ margin: '4px 0 0 0', fontSize: '13px', color: '#64748b' }}>
                Go Native DIMSE Listener (C-ECHO Verification & C-FIND MWL SCP) dengan Redundansi Deskripsi Tindakan
              </p>
            </div>
          </div>
        </div>

        <div style={{ display: 'flex', gap: '10px' }}>
          <button
            onClick={() => { resetForm(); setModalOpen(true); }}
            style={{
              display: 'flex', alignItems: 'center', gap: '6px',
              backgroundColor: '#0284c7', color: '#fff', border: 'none',
              padding: '9px 16px', borderRadius: '8px', fontSize: '13px', fontWeight: '600',
              cursor: 'pointer', boxShadow: '0 2px 4px rgba(2, 132, 199, 0.2)'
            }}
          >
            <Plus size={16} /> Tambah Order Manual
          </button>

          <button
            onClick={() => setServerSettingsOpen(!serverSettingsOpen)}
            style={{
              display: 'flex', alignItems: 'center', gap: '6px',
              backgroundColor: '#f1f5f9', color: '#334155', border: '1px solid #cbd5e1',
              padding: '9px 14px', borderRadius: '8px', fontSize: '13px', fontWeight: '500',
              cursor: 'pointer'
            }}
          >
            <Server size={16} /> Server Settings
          </button>

          <button
            onClick={() => { fetchWorklist(); fetchStats(); }}
            style={{
              display: 'flex', alignItems: 'center', gap: '6px',
              backgroundColor: '#ffffff', color: '#64748b', border: '1px solid #e2e8f0',
              padding: '9px 12px', borderRadius: '8px', fontSize: '13px',
              cursor: 'pointer'
            }}
          >
            <RefreshCw size={15} />
          </button>
        </div>
      </div>

      {/* Server Control Drawer */}
      {serverSettingsOpen && (
        <div style={{
          backgroundColor: '#f8fafc', border: '1px solid #e2e8f0', borderRadius: '12px',
          padding: '16px 20px', marginBottom: '24px', display: 'flex', alignItems: 'center',
          justifyContent: 'space-between', flexWrap: 'wrap', gap: '16px'
        }}>
          <div>
            <div style={{ fontWeight: '600', fontSize: '14px', color: '#0f172a' }}>Konfigurasi DICOM DIMSE SCP Listener</div>
            <div style={{ fontSize: '12px', color: '#64748b' }}>
              Mesin modalitas (GE, Siemens, Philips, dsb.) harus diarahkan ke IP host ini dengan port dan AE Title berikut.
            </div>
          </div>
          <div style={{ display: 'flex', alignItems: 'center', gap: '12px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <label style={{ fontSize: '12px', fontWeight: '600', color: '#475569' }}>Port:</label>
              <input
                type="number"
                value={serverPort}
                onChange={(e) => setServerPort(e.target.value)}
                style={{ width: '80px', padding: '6px 10px', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '13px' }}
              />
            </div>
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <label style={{ fontSize: '12px', fontWeight: '600', color: '#475569' }}>AE Title:</label>
              <input
                type="text"
                value={serverAET}
                onChange={(e) => setServerAET(e.target.value)}
                style={{ width: '160px', padding: '6px 10px', borderRadius: '6px', border: '1px solid #cbd5e1', fontSize: '13px' }}
              />
            </div>
            <button
              onClick={() => handleControlServer('restart')}
              style={{
                backgroundColor: '#0f172a', color: '#fff', border: 'none',
                padding: '7px 14px', borderRadius: '6px', fontSize: '12px', fontWeight: '600',
                cursor: 'pointer'
              }}
            >
              Simpan & Restart Server
            </button>
          </div>
        </div>
      )}

      {/* Telemetry Metric Cards */}
      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(220px, 1fr))', gap: '16px', marginBottom: '24px' }}>
        <div style={{ background: '#fff', padding: '16px', borderRadius: '12px', border: '1px solid #e2e8f0', boxShadow: '0 1px 3px rgba(0,0,0,0.05)' }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '8px' }}>
            <span style={{ fontSize: '12px', fontWeight: '600', color: '#64748b' }}>STATUS LISTENER</span>
            <span style={{
              display: 'inline-flex', alignItems: 'center', gap: '4px',
              padding: '2px 8px', borderRadius: '9999px', fontSize: '11px', fontWeight: '600',
              backgroundColor: stats?.running ? '#ecfdf5' : '#fef2f2',
              color: stats?.running ? '#059669' : '#dc2626'
            }}>
              <span style={{ width: '6px', height: '6px', borderRadius: '50%', backgroundColor: stats?.running ? '#10b981' : '#ef4444' }} />
              {stats?.running ? 'ONLINE' : 'STOPPED'}
            </span>
          </div>
          <div style={{ fontSize: '18px', fontWeight: '700', color: '#0f172a' }}>Port {stats?.port || 11112}</div>
          <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px' }}>AET: <code style={{ color: '#0284c7' }}>{stats?.ae_title || 'HEALTH_CONNECT'}</code></div>
          <div style={{ fontSize: '11px', color: '#166534', marginTop: '4px', backgroundColor: '#f0fdf4', padding: '2px 6px', borderRadius: '4px' }}>
            IP LAN Server: <strong>{stats?.server_ips?.join(', ') || '127.0.0.1'}</strong>
          </div>
        </div>

        <div style={{ background: '#fff', padding: '16px', borderRadius: '12px', border: '1px solid #e2e8f0', boxShadow: '0 1px 3px rgba(0,0,0,0.05)' }}>
          <div style={{ fontSize: '12px', fontWeight: '600', color: '#64748b', marginBottom: '8px' }}>C-ECHO VERIFICATION</div>
          <div style={{ fontSize: '24px', fontWeight: '700', color: '#0f172a' }}>{stats?.echo_count || 0}</div>
          <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px' }}>Ping verifikasi dari modalitas</div>
        </div>

        <div style={{ background: '#fff', padding: '16px', borderRadius: '12px', border: '1px solid #e2e8f0', boxShadow: '0 1px 3px rgba(0,0,0,0.05)' }}>
          <div style={{ fontSize: '12px', fontWeight: '600', color: '#64748b', marginBottom: '8px' }}>C-FIND MWL QUERIES</div>
          <div style={{ fontSize: '24px', fontWeight: '700', color: '#0284c7' }}>{stats?.find_count || 0}</div>
          <div style={{ fontSize: '12px', color: '#64748b', marginTop: '2px' }}>{stats?.matches_returned || 0} pasien dikirim ke modalitas</div>
        </div>

        <div style={{ background: '#fff', padding: '16px', borderRadius: '12px', border: '1px solid #e2e8f0', boxShadow: '0 1px 3px rgba(0,0,0,0.05)' }}>
          <div style={{ fontSize: '12px', fontWeight: '600', color: '#64748b', marginBottom: '8px' }}>MODALITAS TERAKHIR</div>
          <div style={{ fontSize: '16px', fontWeight: '700', color: '#0f172a', whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis' }}>
            {stats?.last_client_ae_title || 'Belum ada koneksi'}
          </div>
          <div style={{ fontSize: '11px', color: '#94a3b8', marginTop: '4px' }}>
            {stats?.last_activity_at ? new Date(stats.last_activity_at).toLocaleTimeString() : '-'}
          </div>
        </div>
      </div>

      {/* Filter Bar */}
      <div style={{
        background: '#fff', padding: '16px', borderRadius: '12px', border: '1px solid #e2e8f0',
        display: 'flex', flexWrap: 'wrap', gap: '12px', alignItems: 'center', marginBottom: '20px'
      }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
          <Filter size={15} color="#64748b" />
          <span style={{ fontSize: '13px', fontWeight: '600', color: '#475569' }}>Filter:</span>
        </div>

        <div style={{ minWidth: '150px' }}>
          <input
            type="text"
            placeholder="Cari RM, Nama, Order..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ width: '100%', padding: '7px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
          />
        </div>

        <div>
          <select
            value={modalityFilter}
            onChange={(e) => setModalityFilter(e.target.value)}
            style={{ padding: '7px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px', backgroundColor: '#fff' }}
          >
            <option value="ALL">Semua Modalitas</option>
            <option value="DX">DX (Digital Radiography)</option>
            <option value="CR">CR (Computed Radiography)</option>
            <option value="CT">CT (Computed Tomography)</option>
            <option value="MR">MR (Magnetic Resonance)</option>
            <option value="US">US (Ultrasound)</option>
            <option value="MG">MG (Mammography)</option>
          </select>
        </div>

        <div>
          <select
            value={statusFilter}
            onChange={(e) => setStatusFilter(e.target.value)}
            style={{ padding: '7px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px', backgroundColor: '#fff' }}
          >
            <option value="ALL">Semua Status</option>
            <option value="SCHEDULED">SCHEDULED (Antre)</option>
            <option value="IN_PROGRESS">IN_PROGRESS (Sedang Diperiksa)</option>
            <option value="COMPLETED">COMPLETED (Selesai)</option>
            <option value="CANCELLED">CANCELLED (Batal)</option>
          </select>
        </div>

        <div style={{ marginLeft: 'auto', fontSize: '13px', color: '#64748b' }}>
          Total Antrean: <strong>{items.length}</strong> pasien
        </div>
      </div>

      {/* Worklist Table */}
      <div style={{ background: '#fff', borderRadius: '12px', border: '1px solid #e2e8f0', overflow: 'hidden', boxShadow: '0 1px 3px rgba(0,0,0,0.05)' }}>
        <table style={{ width: '100%', borderCollapse: 'collapse', textAlign: 'left', fontSize: '13px' }}>
          <thead>
            <tr style={{ backgroundColor: '#f8fafc', borderBottom: '1px solid #e2e8f0', color: '#475569', fontWeight: '600' }}>
              <th style={{ padding: '12px 16px' }}>Pasien / RM</th>
              <th style={{ padding: '12px 16px' }}>No. Order / Accession</th>
              <th style={{ padding: '12px 16px' }}>Modalitas</th>
              <th style={{ padding: '12px 16px' }}>Tindakan (Auto-Copied Tags)</th>
              <th style={{ padding: '12px 16px' }}>Jadwal Periksa</th>
              <th style={{ padding: '12px 16px' }}>Target Alat (AET)</th>
              <th style={{ padding: '12px 16px' }}>Status</th>
              <th style={{ padding: '12px 16px', textAlign: 'right' }}>Aksi</th>
            </tr>
          </thead>
          <tbody>
            {loading ? (
              <tr>
                <td colSpan={8} style={{ padding: '36px', textAlign: 'center', color: '#94a3b8' }}>
                  Memuat antrean worklist...
                </td>
              </tr>
            ) : items.length === 0 ? (
              <tr>
                <td colSpan={8} style={{ padding: '48px', textAlign: 'center', color: '#94a3b8' }}>
                  <div style={{ marginBottom: '8px' }}><Scan size={32} color="#cbd5e1" /></div>
                  Tidak ada antrean pemeriksaan yang cocok dengan filter.
                </td>
              </tr>
            ) : (
              items.map((item) => (
                <tr key={item.id} style={{ borderBottom: '1px solid #f1f5f9' }}>
                  <td style={{ padding: '12px 16px' }}>
                    <div style={{ fontWeight: '600', color: '#0f172a' }}>{item.patient_name}</div>
                    <div style={{ fontSize: '11px', color: '#64748b' }}>
                      RM: <code style={{ backgroundColor: '#f1f5f9', padding: '1px 4px', borderRadius: '4px' }}>{item.patient_id}</code>
                      {item.gender ? ` • ${item.gender}` : ''}
                    </div>
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    <code style={{ fontSize: '12px', fontWeight: '600', color: '#0284c7' }}>{item.accession_number}</code>
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    <span style={{
                      padding: '3px 8px', borderRadius: '6px', fontSize: '11px', fontWeight: '700',
                      backgroundColor: item.modality === 'CT' ? '#ede9fe' : item.modality === 'MR' ? '#fce7f3' : '#e0f2fe',
                      color: item.modality === 'CT' ? '#6d28d9' : item.modality === 'MR' ? '#be185d' : '#0369a1'
                    }}>
                      {item.modality}
                    </span>
                  </td>
                  <td style={{ padding: '12px 16px', maxWidth: '300px' }}>
                    <div style={{ fontWeight: '500', color: '#1e293b' }}>{item.procedure_desc}</div>
                    <div style={{ display: 'flex', gap: '4px', marginTop: '4px', flexWrap: 'wrap' }}>
                      <span title="Tersalin ke Study Description (0008,1030)" style={{ fontSize: '10px', backgroundColor: '#f0fdf4', color: '#166534', padding: '1px 5px', borderRadius: '4px', border: '1px solid #bbf7d0' }}>
                        ✓ Study Desc
                      </span>
                      <span title="Tersalin ke Req Proc Description (0032,1060)" style={{ fontSize: '10px', backgroundColor: '#f0fdf4', color: '#166534', padding: '1px 5px', borderRadius: '4px', border: '1px solid #bbf7d0' }}>
                        ✓ Req Proc
                      </span>
                      <span title="Tersalin ke Series Description (0008,103E)" style={{ fontSize: '10px', backgroundColor: '#f0fdf4', color: '#166534', padding: '1px 5px', borderRadius: '4px', border: '1px solid #bbf7d0' }}>
                        ✓ Series Desc
                      </span>
                    </div>
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    <div style={{ color: '#334155' }}>{item.scheduled_date}</div>
                    <div style={{ fontSize: '11px', color: '#94a3b8' }}>{item.scheduled_time || '00:00:00'}</div>
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    <code style={{ fontSize: '11px', backgroundColor: '#f8fafc', padding: '2px 6px', borderRadius: '4px', border: '1px solid #e2e8f0' }}>
                      {item.station_ae_title || '*'}
                    </code>
                  </td>
                  <td style={{ padding: '12px 16px' }}>
                    <span style={{
                      display: 'inline-block', padding: '3px 8px', borderRadius: '9999px', fontSize: '11px', fontWeight: '600',
                      backgroundColor: item.status === 'SCHEDULED' ? '#e0f2fe' : item.status === 'IN_PROGRESS' ? '#fef3c7' : item.status === 'COMPLETED' ? '#dcfce7' : '#fee2e2',
                      color: item.status === 'SCHEDULED' ? '#0369a1' : item.status === 'IN_PROGRESS' ? '#b45309' : item.status === 'COMPLETED' ? '#15803d' : '#b91c1c'
                    }}>
                      {item.status}
                    </span>
                  </td>
                  <td style={{ padding: '12px 16px', textAlign: 'right' }}>
                    <div style={{ display: 'inline-flex', gap: '6px' }}>
                      {item.status === 'SCHEDULED' && (
                        <button
                          title="Tandai Selesai"
                          onClick={() => handleUpdateStatus(item.id, 'COMPLETED')}
                          style={{ padding: '4px 8px', borderRadius: '6px', border: '1px solid #cbd5e1', backgroundColor: '#fff', cursor: 'pointer', color: '#16a34a' }}
                        >
                          <Check size={14} />
                        </button>
                      )}
                      <button
                        title="Lihat Detail Tag DICOM"
                        onClick={() => setDetailModalItem(item)}
                        style={{ padding: '4px 8px', borderRadius: '6px', border: '1px solid #cbd5e1', backgroundColor: '#fff', cursor: 'pointer', color: '#0284c7' }}
                      >
                        <Eye size={14} />
                      </button>
                      <button
                        title="Edit Order"
                        onClick={() => openEditModal(item)}
                        style={{ padding: '4px 8px', borderRadius: '6px', border: '1px solid #cbd5e1', backgroundColor: '#fff', cursor: 'pointer', color: '#475569' }}
                      >
                        <Edit2 size={14} />
                      </button>
                      <button
                        title="Hapus"
                        onClick={() => handleDelete(item.id)}
                        style={{ padding: '4px 8px', borderRadius: '6px', border: '1px solid #cbd5e1', backgroundColor: '#fff', cursor: 'pointer', color: '#dc2626' }}
                      >
                        <Trash2 size={14} />
                      </button>
                    </div>
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Modal Tambah / Edit Order */}
      {modalOpen && (
        <div style={{
          position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(15, 23, 42, 0.6)', backdropFilter: 'blur(4px)',
          display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999, padding: '20px'
        }}>
          <div style={{
            backgroundColor: '#fff', borderRadius: '16px', width: '100%', maxWidth: '640px',
            maxHeight: '90vh', overflowY: 'auto', boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.1)',
            padding: '24px'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '20px', borderBottom: '1px solid #f1f5f9', paddingBottom: '12px' }}>
              <h3 style={{ margin: 0, fontSize: '18px', fontWeight: '700', color: '#0f172a' }}>
                {formData.id ? 'Edit Order Pemeriksaan Radiologi' : 'Tambah Order Pemeriksaan (Simulasi SIMRS)'}
              </h3>
              <button onClick={() => setModalOpen(false)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#64748b' }}>
                <X size={20} />
              </button>
            </div>

            <form onSubmit={handleSaveOrder}>
              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '16px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>No. Rekam Medis (Patient ID) *</label>
                  <input
                    type="text"
                    required
                    value={formData.patient_id}
                    onChange={(e) => setFormData({ ...formData, patient_id: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>No. Order / Accession *</label>
                  <input
                    type="text"
                    required
                    value={formData.accession_number}
                    onChange={(e) => setFormData({ ...formData, accession_number: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                  />
                </div>
              </div>

              <div style={{ marginBottom: '16px' }}>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>Nama Pasien *</label>
                <input
                  type="text"
                  required
                  placeholder="Format: Bambang Sutrisno atau Sutrisno^Bambang"
                  value={formData.patient_name}
                  onChange={(e) => setFormData({ ...formData, patient_name: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                />
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr 1fr', gap: '16px', marginBottom: '16px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>Tgl Lahir (YYYYMMDD)</label>
                  <input
                    type="text"
                    placeholder="19900101"
                    value={formData.birth_date}
                    onChange={(e) => setFormData({ ...formData, birth_date: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>Jenis Kelamin</label>
                  <select
                    value={formData.gender}
                    onChange={(e) => setFormData({ ...formData, gender: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px', backgroundColor: '#fff' }}
                  >
                    <option value="M">Laki-laki (M)</option>
                    <option value="F">Perempuan (F)</option>
                    <option value="O">Lainnya (O)</option>
                  </select>
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>Modalitas</label>
                  <select
                    value={formData.modality}
                    onChange={(e) => setFormData({ ...formData, modality: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px', backgroundColor: '#fff' }}
                  >
                    <option value="DX">DX (X-Ray DR)</option>
                    <option value="CR">CR (Computed Rad)</option>
                    <option value="CT">CT (CT Scan)</option>
                    <option value="MR">MR (MRI)</option>
                    <option value="US">US (USG)</option>
                    <option value="MG">MG (Mamografi)</option>
                  </select>
                </div>
              </div>

              <div style={{ marginBottom: '16px' }}>
                <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>
                  Nama Tindakan / Prosedur Pemeriksaan *
                </label>
                <input
                  type="text"
                  required
                  placeholder="Misal: Rontgen Thorax PA/AP atau CT Kepala Polos"
                  value={formData.procedure_desc}
                  onChange={(e) => setFormData({ ...formData, procedure_desc: e.target.value })}
                  style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                />
                <div style={{ marginTop: '6px', fontSize: '11px', color: '#16a34a', backgroundColor: '#f0fdf4', padding: '6px 10px', borderRadius: '6px', border: '1px solid #bbf7d0' }}>
                  ℹ️ <strong>Aturan Redundansi Aktif:</strong> Deskripsi ini otomatis disalin ke <strong>Study Description (0008,1030)</strong>, <strong>Requested Procedure Description (0032,1060)</strong>, dan <strong>Series Description (0008,103E)</strong>.
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '16px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>Tgl Periksa (YYYYMMDD)</label>
                  <input
                    type="text"
                    value={formData.scheduled_date}
                    onChange={(e) => setFormData({ ...formData, scheduled_date: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>Jam Periksa (HHMMSS)</label>
                  <input
                    type="text"
                    value={formData.scheduled_time}
                    onChange={(e) => setFormData({ ...formData, scheduled_time: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                  />
                </div>
              </div>

              <div style={{ display: 'grid', gridTemplateColumns: '1fr 1fr', gap: '16px', marginBottom: '24px' }}>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>Target Station AE Title</label>
                  <input
                    type="text"
                    placeholder="* (semua alat) atau nama AET"
                    value={formData.station_ae_title}
                    onChange={(e) => setFormData({ ...formData, station_ae_title: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                  />
                </div>
                <div>
                  <label style={{ display: 'block', fontSize: '12px', fontWeight: '600', color: '#475569', marginBottom: '4px' }}>Dokter Pengirim</label>
                  <input
                    type="text"
                    value={formData.physician_name}
                    onChange={(e) => setFormData({ ...formData, physician_name: e.target.value })}
                    style={{ width: '100%', padding: '8px 12px', borderRadius: '8px', border: '1px solid #cbd5e1', fontSize: '13px' }}
                  />
                </div>
              </div>

              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: '12px' }}>
                <button
                  type="button"
                  onClick={() => setModalOpen(false)}
                  style={{
                    padding: '8px 16px', borderRadius: '8px', border: '1px solid #cbd5e1',
                    backgroundColor: '#fff', fontSize: '13px', cursor: 'pointer', color: '#475569'
                  }}
                >
                  Batal
                </button>
                <button
                  type="submit"
                  style={{
                    padding: '8px 18px', borderRadius: '8px', border: 'none',
                    backgroundColor: '#0284c7', color: '#fff', fontSize: '13px', fontWeight: '600',
                    cursor: 'pointer'
                  }}
                >
                  Simpan Order Worklist
                </button>
              </div>
            </form>
          </div>
        </div>
      )}

      {/* Modal Detail Tag DICOM */}
      {detailModalItem && (
        <div style={{
          position: 'fixed', top: 0, left: 0, right: 0, bottom: 0,
          backgroundColor: 'rgba(15, 23, 42, 0.6)', backdropFilter: 'blur(4px)',
          display: 'flex', alignItems: 'center', justifyContent: 'center', zIndex: 9999, padding: '20px'
        }}>
          <div style={{
            backgroundColor: '#fff', borderRadius: '16px', width: '100%', maxWidth: '720px',
            maxHeight: '90vh', overflowY: 'auto', boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.1)',
            padding: '24px'
          }}>
            <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '16px', borderBottom: '1px solid #f1f5f9', paddingBottom: '12px' }}>
              <div>
                <h3 style={{ margin: 0, fontSize: '16px', fontWeight: '700', color: '#0f172a' }}>
                  Dataset Tag DICOM untuk Mesin Modalitas (C-FIND RSP)
                </h3>
                <div style={{ fontSize: '12px', color: '#64748b' }}>
                  Preview data biner DICOM yang akan diterima oleh konsol operator radiologi
                </div>
              </div>
              <button onClick={() => setDetailModalItem(null)} style={{ background: 'none', border: 'none', cursor: 'pointer', color: '#64748b' }}>
                <X size={20} />
              </button>
            </div>

            <div style={{ backgroundColor: '#0f172a', color: '#f8fafc', padding: '16px', borderRadius: '10px', fontSize: '12px', fontFamily: 'monospace', overflowX: 'auto', marginBottom: '16px' }}>
              <div>(0008,0005) Specific Character Set: <span style={{ color: '#38bdf8' }}>ISO_IR 100</span></div>
              <div>(0008,0050) Accession Number: <span style={{ color: '#38bdf8' }}>{detailModalItem.accession_number}</span></div>
              <div>(0008,0090) Referring Physician: <span style={{ color: '#38bdf8' }}>{detailModalItem.physician_name}</span></div>
              <div style={{ color: '#4ade80', fontWeight: 'bold' }}>
                (0008,1030) Study Description: <span style={{ color: '#fef08a' }}>{detailModalItem.study_desc || detailModalItem.procedure_desc}</span> [COPIED]
              </div>
              <div>(0010,0010) Patient's Name: <span style={{ color: '#38bdf8' }}>{detailModalItem.patient_name}</span></div>
              <div>(0010,0020) Patient ID: <span style={{ color: '#38bdf8' }}>{detailModalItem.patient_id}</span></div>
              <div>(0010,0030) Patient's Birth Date: <span style={{ color: '#38bdf8' }}>{detailModalItem.birth_date}</span></div>
              <div>(0010,0040) Patient's Sex: <span style={{ color: '#38bdf8' }}>{detailModalItem.gender}</span></div>
              <div>(0020,000D) Study Instance UID: <span style={{ color: '#f43f5e' }}>{detailModalItem.study_uid}</span></div>
              <div style={{ color: '#4ade80', fontWeight: 'bold' }}>
                (0032,1060) Requested Procedure Description: <span style={{ color: '#fef08a' }}>{detailModalItem.req_proc_desc || detailModalItem.procedure_desc}</span> [COPIED]
              </div>
              <div style={{ marginTop: '8px', color: '#94a3b8' }}>Sequence (0040,0100) Scheduled Procedure Step Sequence:</div>
              <div style={{ paddingLeft: '16px' }}>
                <div>(0008,0060) Modality: <span style={{ color: '#38bdf8' }}>{detailModalItem.modality}</span></div>
                <div>(0040,0001) Scheduled Station AE Title: <span style={{ color: '#38bdf8' }}>{detailModalItem.station_ae_title}</span></div>
                <div>(0040,0002) Scheduled Step Start Date: <span style={{ color: '#38bdf8' }}>{detailModalItem.scheduled_date}</span></div>
                <div>(0040,0003) Scheduled Step Start Time: <span style={{ color: '#38bdf8' }}>{detailModalItem.scheduled_time}</span></div>
                <div>(0040,0006) Scheduled Performing Physician: <span style={{ color: '#38bdf8' }}>{detailModalItem.physician_name}</span></div>
                <div style={{ color: '#4ade80', fontWeight: 'bold' }}>
                  (0040,0007) Scheduled Procedure Step Description: <span style={{ color: '#fef08a' }}>{detailModalItem.procedure_desc}</span> [SOURCE]
                </div>
                <div style={{ color: '#4ade80', fontWeight: 'bold' }}>
                  (0008,103E) Series Description: <span style={{ color: '#fef08a' }}>{detailModalItem.series_desc || detailModalItem.procedure_desc}</span> [COPIED]
                </div>
                <div>(0040,0009) Scheduled Step ID: <span style={{ color: '#38bdf8' }}>SPS_{detailModalItem.accession_number}</span></div>
              </div>
            </div>

            <div style={{ display: 'flex', justifyContent: 'flex-end' }}>
              <button
                onClick={() => setDetailModalItem(null)}
                style={{
                  padding: '8px 18px', borderRadius: '8px', border: '1px solid #cbd5e1',
                  backgroundColor: '#f1f5f9', fontSize: '13px', fontWeight: '600', cursor: 'pointer'
                }}
              >
                Tutup Preview
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
