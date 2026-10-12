import sqlite3 from 'sqlite3';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import fs from 'node:fs';

// Simple .env parser to avoid requiring 'dotenv' module if not installed
try {
  const envFile = fs.readFileSync('.env', 'utf8');
  envFile.split('\n').forEach(line => {
    const match = /^([^=]+)=(.*)$/.exec(line);
    if (match) process.env[match[1].trim()] = match[2].trim();
  });
} catch(e) {
  // .env file is optional, safely ignore if not found
}

const dbType = process.env.DB_TYPE || 'sqlite'; // Supported: 'sqlite', 'mysql', 'postgres'
let pool = null;
let sqliteDb = null;

async function connectDb() {
  if (dbType === 'postgres') {
    const pg = await import('pg');
    pool = new pg.default.Pool({
      connectionString: process.env.DATABASE_URL || 'postgresql://user:pass@localhost:5432/middleware'
    });
    console.log('Connected to PostgreSQL database.');
  } else if (dbType === 'mysql') {
    const mysql = await import('mysql2/promise');
    pool = mysql.createPool(process.env.DATABASE_URL || 'mysql://user:pass@localhost:3306/middleware');
    console.log('Connected to MySQL database.');
  } else {
    const __dirname = dirname(fileURLToPath(import.meta.url));
    const dbPath = join(__dirname, '../instance/middleware.db');
    sqliteDb = new sqlite3.Database(dbPath, (err) => {
      if (err) console.error('Error opening database', err.message);
      else console.log('Connected to SQLite database.');
    });
  }
}

// Helper for PostgreSQL: Converts SQLite '?' placeholders to '$1, $2' etc.
function convertSqlForPg(sql) {
  let i = 1;
  return sql.replaceAll('?', () => `$${i++}`);
}

export const dbRun = async (sql, params = []) => {
  if (dbType === 'postgres') {
    return pool.query(convertSqlForPg(sql), params);
  } else if (dbType === 'mysql') {
    return pool.execute(sql, params);
  } else {
    return new Promise((resolve, reject) => {
      sqliteDb.run(sql, params, function (err) {
        if (err) reject(err); else resolve(this);
      });
    });
  }
};

export const dbGet = async (sql, params = []) => {
  if (dbType === 'postgres') {
    const { rows } = await pool.query(convertSqlForPg(sql), params);
    return rows[0];
  } else if (dbType === 'mysql') {
    const [rows] = await pool.query(sql, params);
    return rows[0];
  } else {
    return new Promise((resolve, reject) => {
      sqliteDb.get(sql, params, (err, row) => {
        if (err) reject(err); else resolve(row);
      });
    });
  }
};

export const dbAll = async (sql, params = []) => {
  if (dbType === 'postgres') {
    const { rows } = await pool.query(convertSqlForPg(sql), params);
    return rows;
  } else if (dbType === 'mysql') {
    const [rows] = await pool.query(sql, params);
    return rows;
  } else {
    return new Promise((resolve, reject) => {
      sqliteDb.all(sql, params, (err, rows) => {
        if (err) reject(err); else resolve(rows);
      });
    });
  }
};

async function initDb() {
  try {
    // MySQL works better with VARCHAR for primary and foreign keys
    const varchar = (dbType === 'mysql') ? 'VARCHAR(100)' : 'TEXT';

    await dbRun(`CREATE TABLE IF NOT EXISTS route_groups (
      id ${varchar} PRIMARY KEY,
      name ${varchar} UNIQUE,
      description TEXT
    )`);

    await dbRun(`CREATE TABLE IF NOT EXISTS routes (
      id ${varchar} PRIMARY KEY,
      name ${varchar} NOT NULL,
      description TEXT,
      group_name TEXT,
      active INTEGER DEFAULT 1
    )`);

    await dbRun(`CREATE TABLE IF NOT EXISTS endpoints (
      id ${varchar} PRIMARY KEY,
      route_id ${varchar},
      role ${varchar},
      protocol ${varchar},
      config TEXT,
      format ${varchar},
      FOREIGN KEY (route_id) REFERENCES routes(id)
    )`);

    await dbRun(`CREATE TABLE IF NOT EXISTS transformers (
      id ${varchar} PRIMARY KEY,
      route_id ${varchar},
      script TEXT,
      inbound_template TEXT,
      outbound_template TEXT,
      FOREIGN KEY (route_id) REFERENCES routes(id)
    )`);

    await dbRun(`CREATE TABLE IF NOT EXISTS message_logs (
      id ${varchar} PRIMARY KEY,
      route_id ${varchar},
      source_protocol ${varchar},
      target_protocol ${varchar},
      inbound_data TEXT,
      outbound_data TEXT,
      response_data TEXT,
      status ${varchar},
      error_message TEXT,
      created_at ${varchar},
      FOREIGN KEY (route_id) REFERENCES routes(id)
    )`);
    
    // Add column if migrating from old schema
    try {
      await dbRun(`ALTER TABLE message_logs ADD COLUMN response_data TEXT`);
    } catch (e) {
      // Ignore, column already exists
    }
    
    await migrateDb();
  } catch (error) {
    console.error("Database initialization failed:", error);
  }
}

async function migrateDb() {
  console.log("Checking database migrations...");
  try {
    let colNames = [];
    if (dbType === 'postgres') {
      const rows = await dbAll("SELECT column_name FROM information_schema.columns WHERE table_name = 'transformers'");
      colNames = rows.map(r => r.column_name);
    } else if (dbType === 'mysql') {
      const rows = await dbAll("SHOW COLUMNS FROM transformers");
      colNames = rows.map(r => r.Field);
    } else {
      const cols = await dbAll("PRAGMA table_info(transformers)");
      colNames = cols.map(c => c.name);
    }
    
    if (!colNames.includes('inbound_template')) {
      await dbRun("ALTER TABLE transformers ADD COLUMN inbound_template TEXT");
      console.log("✅ Migration applied: Added 'inbound_template' to 'transformers'");
    }
    
    if (!colNames.includes('outbound_template')) {
      await dbRun("ALTER TABLE transformers ADD COLUMN outbound_template TEXT");
      console.log("✅ Migration applied: Added 'outbound_template' to 'transformers'");
    }
    
  } catch (error) {
    console.error("Migration failed:", error);
  }
}

// Start connection and initialization automatically
try {
  await connectDb();
  await initDb();
} catch (err) {
  console.error(err);
}
