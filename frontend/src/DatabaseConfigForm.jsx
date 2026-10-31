import React, { useState, useEffect } from 'react';
import { DB_TEMPLATES } from './dbTemplates';

export default function DatabaseConfigForm({
  driver = 'postgres',
  dsn = '',
  onChangeDriver,
  onChangeDsn,
  pollInterval = '60',
  onChangePollInterval,
  query = '',
  onChangeQuery,
  postQuery,
  onChangePostQuery,
  showPollerFields = true,
  isTarget = false
}) {
  const [showBuilder, setShowBuilder] = useState(false);
  const currentTemplate = DB_TEMPLATES[driver] || DB_TEMPLATES.postgres;

  // Form state for interactive builder
  const [builderState, setBuilderState] = useState({
    host: 'localhost',
    port: currentTemplate.defaultPort,
    user: currentTemplate.defaultUser,
    password: '',
    database: currentTemplate.defaultDatabase,
    options: currentTemplate.defaultOptions
  });

  // When driver changes, update builder defaults
  useEffect(() => {
    const tmpl = DB_TEMPLATES[driver] || DB_TEMPLATES.postgres;
    setBuilderState(prev => ({
      ...prev,
      port: tmpl.defaultPort,
      user: tmpl.defaultUser,
      database: tmpl.defaultDatabase,
      options: tmpl.defaultOptions
    }));
  }, [driver]);

  const handleDriverChange = (newDriver) => {
    onChangeDriver(newDriver);
    const tmpl = DB_TEMPLATES[newDriver] || DB_TEMPLATES.postgres;
    // Auto-fill DSN if currently empty or matching another template
    if (!dsn || Object.values(DB_TEMPLATES).some(t => t.sample === dsn || t.presets.some(p => p.dsn === dsn))) {
      onChangeDsn(tmpl.sample);
    }
  };

  const applyBuilder = () => {
    if (currentTemplate.build) {
      const generatedDsn = currentTemplate.build(builderState);
      onChangeDsn(generatedDsn);
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: '1rem' }}>
      {/* Driver & Poll Interval Row */}
      <div style={{ display: 'grid', gridTemplateColumns: showPollerFields ? '1fr 160px' : '1fr', gap: '1rem' }}>
        <div className="form-group" style={{ margin: 0 }}>
          <label style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center' }}>
            <span style={{ fontWeight: 600 }}>Database Driver / Vendor *</span>
            <span style={{ fontSize: '0.75rem', color: 'var(--primary)', fontWeight: 500 }}>
              {currentTemplate.name}
            </span>
          </label>
          <select
            className="form-control"
            value={driver}
            onChange={e => handleDriverChange(e.target.value)}
            style={{ fontWeight: 600 }}
          >
            <option value="postgres">🐘 PostgreSQL (Satu Sehat, PACS, OpenMRS, HIS)</option>
            <option value="mysql">🐬 MySQL (SIMRS Khanza, Medin, Kemenkes)</option>
            <option value="mariadb">🦭 MariaDB (Kompatibel MySQL Protocol)</option>
            <option value="sqlserver">🪟 Microsoft SQL Server (MSSQL 2012-2022)</option>
            <option value="sqlite">📁 SQLite (Internal Health-Connect / File .db)</option>
            <option value="odbc">⚡ InterSystems IRIS / Caché (TrakCare ODBC)</option>
            <option value="oracle">🏛️ Oracle Database (11g/12c/19c/21c)</option>
            <option value="hdb">🔷 SAP HANA (Hospital ERP & Healthcare)</option>
          </select>
        </div>

        {showPollerFields && onChangePollInterval && (
          <div className="form-group" style={{ margin: 0 }}>
            <label style={{ fontWeight: 600 }}>Poll Interval (Sec)</label>
            <input
              type="number"
              className="form-control"
              min="1"
              placeholder="60"
              value={pollInterval}
              onChange={e => onChangePollInterval(e.target.value)}
            />
          </div>
        )}
      </div>

      {/* Connection String (DSN) Field with Quick Presets */}
      <div className="form-group" style={{ margin: 0 }}>
        <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.4rem' }}>
          <label style={{ margin: 0, fontWeight: 600 }}>
            Connection String (DSN) *
          </label>
          <button
            type="button"
            className="btn btn-outline"
            style={{ fontSize: '0.75rem', padding: '0.2rem 0.5rem', display: 'flex', alignItems: 'center', gap: '0.3rem' }}
            onClick={() => setShowBuilder(!showBuilder)}
          >
            {showBuilder ? '✕ Close Builder' : '🛠️ Open DSN Builder'}
          </button>
        </div>

        <input
          type="text"
          className="form-control"
          style={{ fontFamily: 'monospace', fontSize: '0.85rem', backgroundColor: 'var(--bg)', color: 'var(--text)' }}
          placeholder={currentTemplate.placeholder}
          value={dsn || ''}
          onChange={e => onChangeDsn(e.target.value)}
          required
        />

        {/* Syntax Hint Box */}
        <div style={{ fontSize: '0.72rem', color: 'var(--text-muted)', marginTop: '0.35rem', display: 'flex', alignItems: 'center', gap: '0.4rem' }}>
          <span style={{ fontWeight: 600, color: 'var(--primary)' }}>Format:</span>
          <code style={{ fontSize: '0.72rem', backgroundColor: 'rgba(99, 102, 241, 0.08)', padding: '0.1rem 0.3rem', borderRadius: '4px' }}>
            {currentTemplate.syntax}
          </code>
        </div>

        {/* Quick Presets Buttons */}
        <div style={{ display: 'flex', flexWrap: 'wrap', gap: '0.4rem', marginTop: '0.6rem', alignItems: 'center' }}>
          <span style={{ fontSize: '0.75rem', fontWeight: 600, color: 'var(--text-muted)' }}>⚡ Preset Cepat:</span>
          {currentTemplate.presets.map((preset, idx) => (
            <button
              key={idx}
              type="button"
              className="btn btn-outline"
              style={{
                fontSize: '0.72rem',
                padding: '0.15rem 0.6rem',
                borderRadius: '12px',
                borderColor: dsn === preset.dsn ? 'var(--primary)' : 'var(--border)',
                backgroundColor: dsn === preset.dsn ? 'rgba(99, 102, 241, 0.15)' : 'transparent',
                fontWeight: dsn === preset.dsn ? 600 : 400
              }}
              onClick={() => onChangeDsn(preset.dsn)}
              title={`Isi DSN: ${preset.dsn}`}
            >
              {preset.label}
            </button>
          ))}
          <button
            type="button"
            className="btn btn-outline"
            style={{ fontSize: '0.72rem', padding: '0.15rem 0.6rem', borderRadius: '12px', color: '#10B981', borderColor: 'rgba(16, 185, 129, 0.3)' }}
            onClick={() => onChangeDsn(currentTemplate.sample)}
          >
            📋 Reset ke Default Sample
          </button>
        </div>
      </div>

      {/* Interactive Builder Panel */}
      {showBuilder && (
        <div style={{
          padding: '1rem',
          backgroundColor: 'var(--bg-card)',
          borderRadius: '8px',
          border: '1px solid var(--primary)',
          boxShadow: '0 4px 12px rgba(0,0,0,0.05)'
        }}>
          <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '0.75rem' }}>
            <span style={{ fontSize: '0.85rem', fontWeight: 700, color: 'var(--primary)' }}>
              🛠️ Interactive Connection String Builder ({currentTemplate.name})
            </span>
            <span style={{ fontSize: '0.75rem', color: 'var(--text-muted)' }}>
              Isi parameter lalu klik Apply
            </span>
          </div>

          {driver === 'sqlite' ? (
            <div className="form-group" style={{ margin: 0 }}>
              <label style={{ fontSize: '0.8rem' }}>Database File Path</label>
              <input
                type="text"
                className="form-control"
                placeholder="middleware.db or /path/to/simrs.db"
                value={builderState.database}
                onChange={e => setBuilderState({ ...builderState, database: e.target.value })}
              />
            </div>
          ) : (
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))', gap: '0.75rem' }}>
              <div className="form-group" style={{ margin: 0 }}>
                <label style={{ fontSize: '0.78rem' }}>Host / IP Address</label>
                <input
                  type="text"
                  className="form-control"
                  style={{ fontSize: '0.85rem' }}
                  placeholder="localhost or 192.168.1.100"
                  value={builderState.host}
                  onChange={e => setBuilderState({ ...builderState, host: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ margin: 0 }}>
                <label style={{ fontSize: '0.78rem' }}>Port</label>
                <input
                  type="text"
                  className="form-control"
                  style={{ fontSize: '0.85rem' }}
                  placeholder={currentTemplate.defaultPort}
                  value={builderState.port}
                  onChange={e => setBuilderState({ ...builderState, port: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ margin: 0 }}>
                <label style={{ fontSize: '0.78rem' }}>Database Name / DSN</label>
                <input
                  type="text"
                  className="form-control"
                  style={{ fontSize: '0.85rem' }}
                  placeholder="hospital_db"
                  value={builderState.database}
                  onChange={e => setBuilderState({ ...builderState, database: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ margin: 0 }}>
                <label style={{ fontSize: '0.78rem' }}>Username</label>
                <input
                  type="text"
                  className="form-control"
                  style={{ fontSize: '0.85rem' }}
                  placeholder="user"
                  value={builderState.user}
                  onChange={e => setBuilderState({ ...builderState, user: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ margin: 0 }}>
                <label style={{ fontSize: '0.78rem' }}>Password</label>
                <input
                  type="password"
                  className="form-control"
                  style={{ fontSize: '0.85rem' }}
                  placeholder="password"
                  value={builderState.password}
                  onChange={e => setBuilderState({ ...builderState, password: e.target.value })}
                />
              </div>

              <div className="form-group" style={{ margin: 0 }}>
                <label style={{ fontSize: '0.78rem' }}>Options / SSL</label>
                <input
                  type="text"
                  className="form-control"
                  style={{ fontSize: '0.85rem' }}
                  placeholder="sslmode=disable or parseTime=true"
                  value={builderState.options}
                  onChange={e => setBuilderState({ ...builderState, options: e.target.value })}
                />
              </div>
            </div>
          )}

          <div style={{ marginTop: '0.75rem', display: 'flex', justifyContent: 'flex-end', gap: '0.5rem' }}>
            <button
              type="button"
              className="btn btn-primary"
              style={{ fontSize: '0.8rem', padding: '0.35rem 0.9rem' }}
              onClick={applyBuilder}
            >
              ✓ Generate & Apply ke DSN
            </button>
          </div>
        </div>
      )}

      {/* SQL Query Field */}
      {showPollerFields && onChangeQuery && (
        <div className="form-group" style={{ margin: 0 }}>
          <label style={{ fontWeight: 600 }}>
            {isTarget ? 'Target Exec / Insert Query (Optional)' : 'Default Poller SELECT Query (Optional)'}
          </label>
          <textarea
            className="form-control editor-textarea"
            rows="2"
            style={{ fontSize: '12px' }}
            placeholder={isTarget ? "INSERT INTO orders (id, payload) VALUES (?, ?)" : "SELECT id, norm, nama FROM table WHERE status='new'"}
            value={query || ''}
            onChange={e => onChangeQuery(e.target.value)}
          />
        </div>
      )}

      {/* Post Query Field (Source poller only) */}
      {showPollerFields && onChangePostQuery && (
        <div className="form-group" style={{ margin: 0 }}>
          <label style={{ fontWeight: 600 }}>Post-Process Update Query (Optional)</label>
          <textarea
            className="form-control editor-textarea"
            rows="2"
            style={{ fontSize: '12px' }}
            placeholder="UPDATE orders SET status='processed' WHERE status='new'"
            value={postQuery || ''}
            onChange={e => onChangePostQuery(e.target.value)}
          />
        </div>
      )}
    </div>
  );
}
