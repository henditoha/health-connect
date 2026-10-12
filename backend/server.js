import express from 'express';
import cors from 'cors';
import * as dbModule from './db.js';
import { IntegrationEngine } from './engine.js';
import crypto from 'crypto';
import net from 'net';
import fetch from 'node-fetch';

const app = express();
const port = 3001;

app.use(cors());
// Use text/plain or json depending on content type
app.use(express.text({ type: '*/*' })); 
// Note: We use express.text for all to easily pass raw data to transformers, 
// then parse inside transformer if needed. Or we can use express.json and fallback.

const engine = new IntegrationEngine(dbModule);

// Initialize Engine on start
setTimeout(() => {
  engine.loadActiveRoutes();
}, 1000); // Wait for DB to init

// --- Management APIs (For Frontend Admin) ---

app.get('/api/routes', async (req, res) => {
  try {
    const routes = await dbModule.dbAll("SELECT * FROM routes");
    for (let r of routes) {
      r.endpoints = await dbModule.dbAll("SELECT * FROM endpoints WHERE route_id = ?", [r.id]);
      r.transformer = await dbModule.dbGet("SELECT * FROM transformers WHERE route_id = ?", [r.id]);
    }
    res.json(routes);
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

app.get('/api/groups', async (req, res) => {
  try {
    const groups = await dbModule.dbAll("SELECT * FROM route_groups ORDER BY name ASC");
    res.json(groups || []);
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

app.post('/api/groups', async (req, res) => {
  try {
    const data = JSON.parse(req.body);
    const id = crypto.randomUUID();
    await dbModule.dbRun("INSERT INTO route_groups (id, name, description) VALUES (?, ?, ?)", [id, data.name, data.description || '']);
    res.json({ id, message: "Group created" });
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

app.get('/api/logs', async (req, res) => {
  try {
    const logs = await dbModule.dbAll(`
      SELECT m.*, r.name as route_name 
      FROM message_logs m 
      LEFT JOIN routes r ON m.route_id = r.id 
      ORDER BY m.created_at DESC 
      LIMIT 100
    `);
    res.json(logs);
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

app.post('/api/routes', async (req, res) => {
  try {
    const data = JSON.parse(req.body);
    const id = crypto.randomUUID();
    await dbModule.dbRun("INSERT INTO routes (id, name, description, group_name) VALUES (?, ?, ?, ?)", [id, data.name, data.description, data.group_name || '']);
    
    // Add default source
    const sourceId = crypto.randomUUID();
    await dbModule.dbRun("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, ?, ?, ?, ?)", 
      [sourceId, id, 'source', 'http', 'json', JSON.stringify({ path: `/ingress/${id}` })]);

    // Add default target
    const targetId = crypto.randomUUID();
    await dbModule.dbRun("INSERT INTO endpoints (id, route_id, role, protocol, format, config) VALUES (?, ?, ?, ?, ?, ?)", 
      [targetId, id, 'target', 'http', 'json', JSON.stringify({ url: `http://example.com/api` })]);

    // Add default transformer
    const transformerId = crypto.randomUUID();
    const defaultScript = `// Parse incoming string to JSON if needed\nlet data = JSON.parse(input);\n\n// Transform logic here\n\nreturn JSON.stringify(data);`;
    await dbModule.dbRun("INSERT INTO transformers (id, route_id, script, inbound_template, outbound_template) VALUES (?, ?, ?, ?, ?)", 
      [transformerId, id, defaultScript, '{\n  "example": "data"\n}', '{\n  "example": "data"\n}']);

    engine.loadActiveRoutes(); // Reload
    res.json({ id, message: "Route created successfully" });
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

app.put('/api/routes/:id', async (req, res) => {
  try {
    const id = req.params.id;
    const data = JSON.parse(req.body);
    
    // Update endpoints
    if (data.source) {
      await dbModule.dbRun("UPDATE endpoints SET protocol = ?, format = ?, config = ? WHERE route_id = ? AND role = 'source'",
        [data.source.protocol, data.source.format, JSON.stringify(data.source.config), id]);
    }
    if (data.target) {
      await dbModule.dbRun("UPDATE endpoints SET protocol = ?, format = ?, config = ? WHERE route_id = ? AND role = 'target'",
        [data.target.protocol, data.target.format, JSON.stringify(data.target.config), id]);
    }
    if (data.transformer) {
      await dbModule.dbRun("UPDATE transformers SET script = ?, inbound_template = ?, outbound_template = ? WHERE route_id = ?",
        [data.transformer.script, data.transformer.inbound_template, data.transformer.outbound_template, id]);
    }
    
    engine.loadActiveRoutes();
    res.json({ message: "Route updated successfully" });
  } catch (e) {
    res.status(500).json({ error: e.message });
  }
});

// --- Ingress API (For external systems to send data to HTTP sources) ---

app.post('/ingress/:routeId', async (req, res) => {
  const { routeId } = req.params;
  
  const route = engine.routes[routeId];
  if (!route) {
    return res.status(404).send({ error: 'Route not found or inactive' });
  }

  const sourceConfig = route.source && route.source.config ? JSON.parse(route.source.config) : {};
  const ackBehavior = sourceConfig.ackBehavior || 'deferred';

  if (ackBehavior === 'immediate') {
    // 1. Immediate ACK (Process in background)
    res.status(200).send("ACK");
    engine.processHttpIngress(routeId, req.body, req.headers).catch(e => console.error(e));
  } else {
    // 2. Deferred ACK (Wait for target response)
    const result = await engine.processHttpIngress(routeId, req.body, req.headers);
    if (result.success) {
      res.status(200).send(result.response || "OK");
    } else {
      res.status(500).send({ error: result.error });
    }
  }
});

app.post('/api/test-connection', async (req, res) => {
  let body = req.body;
  if (typeof body === 'string') {
    try { body = JSON.parse(body); } catch(e) {}
  }
  const { role, protocol, config } = body;
  if (!config) return res.status(400).json({ error: 'Config is required' });

  if (role === 'target') {
    if (protocol === 'tcp') {
      if (!config.port) return res.status(400).json({ error: 'Port is required' });
      const client = new net.Socket();
      client.setTimeout(5000);
      client.on('connect', () => {
        client.destroy();
        res.json({ success: true, message: 'Successfully connected to TCP target' });
      });
      client.on('timeout', () => {
        client.destroy();
        res.status(400).json({ error: 'Connection timed out' });
      });
      client.on('error', (err) => {
        res.status(400).json({ error: err.message });
      });
      client.connect(config.port, config.host || '127.0.0.1');
    } else if (protocol === 'http') {
      try {
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 5000);
        const host = config.host || 'localhost';
        const portStr = config.port ? `:${config.port}` : '';
        const url = config.url || (config.path?.startsWith('http') ? config.path : `http://${host}${portStr}${config.path || ''}`);
        
        await fetch(url, { method: 'GET', signal: controller.signal }).catch(e => {
          throw e; // throw to outer try-catch
        });
        clearTimeout(timeout);
        res.json({ success: true, message: 'Successfully reached HTTP target' });
      } catch (err) {
        res.status(400).json({ error: err.message });
      }
    } else {
      res.json({ success: true, message: 'Mock target is valid' });
    }
  } else if (role === 'source') {
    if (protocol === 'tcp') {
      if (!config.port) return res.status(400).json({ error: 'Port is required' });
      const srv = net.createServer();
      srv.once('error', (err) => {
        res.status(400).json({ error: err.message });
      });
      srv.once('listening', () => {
        srv.close();
        res.json({ success: true, message: 'TCP Port is available for listening' });
      });
      srv.listen(config.port);
    } else {
      res.json({ success: true, message: 'Source config is valid' });
    }
  }
});

app.listen(port, () => {
  console.log(`Middleware Backend running at http://localhost:${port}`);
});
