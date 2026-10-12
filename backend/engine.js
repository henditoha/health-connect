import fetch from 'node-fetch';
import net from 'net';
import crypto from 'crypto';

export class IntegrationEngine {
  constructor(db) {
    this.db = db;
    this.routes = {}; // In-memory active routes
    this.tcpServers = {}; // Active TCP servers
  }

  async loadActiveRoutes() {
    console.log("Loading active integrations...");
    const routes = await this.db.dbAll("SELECT * FROM routes WHERE active = 1");
    for (let route of routes) {
      const endpoints = await this.db.dbAll("SELECT * FROM endpoints WHERE route_id = ?", [route.id]);
      const transformer = await this.db.dbGet("SELECT * FROM transformers WHERE route_id = ?", [route.id]);
      
      this.routes[route.id] = {
        ...route,
        source: endpoints.find(e => e.role === 'source'),
        target: endpoints.find(e => e.role === 'target'),
        transformer: transformer
      };

      // If source is TCP, we need to spin up a server
      const source = this.routes[route.id].source;
      if (source && source.protocol === 'tcp') {
        this.startTcpServer(route.id, source);
      }
    }
  }

  startTcpServer(routeId, source) {
    try {
      const config = JSON.parse(source.config);
      if (this.tcpServers[routeId]) {
         this.tcpServers[routeId].close();
      }

      const server = net.createServer((socket) => {
        let buffer = '';
        socket.on('data', async (data) => {
           buffer += data.toString();
           
           // If we don't have the end of frame yet and it started with MLLP, wait for more data.
           // For simplicity in MVP, if it contains \x1C\x0D or doesn't start with MLLP, process it.
           if (buffer.startsWith('\x0B') && !buffer.includes('\x1C')) {
             return; // Wait for more chunks
           }

           let inputData = buffer;
           let isMllp = false;
           
           // Strip MLLP Framing
           if (inputData.startsWith('\x0B')) {
               isMllp = true;
               inputData = inputData.substring(1);
           }
           if (inputData.includes('\x1C')) {
               inputData = inputData.split('\x1C')[0];
           }
           buffer = ''; // Reset buffer

           const ackBehavior = config.ackBehavior || 'deferred';
           
           const sendResponse = (msg) => {
             socket.write(isMllp ? `\x0B${msg}\x1C\x0D` : msg);
           };

           if (ackBehavior === 'immediate') {
             sendResponse('ACK\r');
             this.executeRoute(routeId, inputData).catch(e => console.error(e));
           } else {
             // Execute route logic and wait for target
             const result = await this.executeRoute(routeId, inputData);
             if (result.success) {
               sendResponse(result.response ? result.response : 'ACK\r');
             } else {
               sendResponse('NACK\r');
             }
           }
        });
      });

      server.listen(config.port, () => {
        console.log(`TCP Source Server listening on port ${config.port} for route ${routeId}`);
      });
      this.tcpServers[routeId] = server;
    } catch (e) {
      console.error(`Error starting TCP server for route ${routeId}`, e);
    }
  }

  async processHttpIngress(routeId, reqBody, reqHeaders) {
    return await this.executeRoute(routeId, reqBody);
  }

  async executeRoute(routeId, inputData) {
    const route = this.routes[routeId];
    if (!route) return { success: false, error: 'Route not found or inactive' };

    console.log(`\n[Route ${route.name}] ⬇️ Received inbound data (${inputData.length} bytes)`);

    let transformedData = inputData;
    let status = 'SUCCESS';
    let errorMsg = '';
    let targetResponse = '';

    try {
      // 1. Run Transformer
      if (route.transformer && route.transformer.script) {
        console.log(`[Route ${route.name}] ⚙️ Executing transformer script...`);
        const transformFn = new Function('input', 'inboundTemplate', 'outboundTemplate', route.transformer.script);
        transformedData = transformFn(inputData, route.transformer.inbound_template, route.transformer.outbound_template);
        console.log(`[Route ${route.name}] ➡️ Transformation complete (${typeof transformedData === 'string' ? transformedData.length : JSON.stringify(transformedData).length} bytes)`);
      }

      // 2. Send to Target
      const target = route.target;
      if (!target) {
         status = 'ERROR'; errorMsg = 'Target not configured';
         return { success: false, error: errorMsg };
      }

      const targetConfig = JSON.parse(target.config);

      if (target.protocol === 'http') {
        const portStr = targetConfig.port ? `:${targetConfig.port}` : '';
        const host = targetConfig.host || 'localhost';
        const url = targetConfig.url || `http://${host}${portStr}${targetConfig.path || ''}`;
        
        let headers = targetConfig.headers || { 'Content-Type': 'application/json' };
        
        if (targetConfig.authType === 'basic') {
          const credentials = Buffer.from(`${targetConfig.username || ''}:${targetConfig.password || ''}`).toString('base64');
          headers['Authorization'] = `Basic ${credentials}`;
        } else if (targetConfig.authType === 'bearer') {
          headers['Authorization'] = `Bearer ${targetConfig.token || ''}`;
        }

        const method = targetConfig.method || 'POST';
        console.log(`[Route ${route.name}] 🌐 Sending to HTTP Target: ${method} ${url}`);
        
        const controller = new AbortController();
        const timeout = setTimeout(() => controller.abort(), 10000); // 10 seconds timeout

        try {
          const response = await fetch(url, {
            method,
            headers,
            body: typeof transformedData === 'string' ? transformedData : JSON.stringify(transformedData),
            signal: controller.signal
          });
          clearTimeout(timeout);
          const resText = await response.text();
          targetResponse = resText;
          console.log(`[Route ${route.name}] ✅ Target Response:`, resText.substring(0, 100) + (resText.length > 100 ? "..." : ""));
          return { success: true, response: resText };
        } catch (fetchErr) {
          clearTimeout(timeout);
          status = 'ERROR'; errorMsg = fetchErr.message;
          console.error(`[Route ${route.name}] ❌ HTTP Target Error:`, fetchErr.message);
          return { success: false, error: fetchErr.message };
        }
      } else if (target.protocol === 'tcp') {
        console.log(`[Route ${route.name}] 🔌 Sending to TCP Target: ${targetConfig.host}:${targetConfig.port}`);
        const tcpResult = await new Promise((resolve) => {
          let timeoutTriggered = false;
          const client = new net.Socket();
          
          const timer = setTimeout(() => {
            timeoutTriggered = true;
            client.destroy();
            console.error(`[Route ${route.name}] ❌ TCP Target Error: Connection timed out`);
            resolve({ success: false, error: 'Connection timed out' });
          }, 10000); // 10 seconds timeout

          client.connect(targetConfig.port, targetConfig.host, () => {
            if (timeoutTriggered) return;
            let payload = typeof transformedData === 'string' ? transformedData : JSON.stringify(transformedData);
            
            if (target.format === 'hl7' || payload.startsWith('MSH')) {
               payload = `\x0B${payload}\x1C\x0D`;
            }
            
            client.write(payload);
          });
          
          let resBuffer = '';
          client.on('data', (data) => {
            if (timeoutTriggered) return;
            resBuffer += data.toString();
            
            if (resBuffer.startsWith('\x0B') && !resBuffer.includes('\x1C')) {
              return; 
            }

            clearTimeout(timer);
            client.destroy();
            
            let resText = resBuffer;
            if (resText.startsWith('\x0B')) resText = resText.substring(1);
            if (resText.includes('\x1C')) resText = resText.split('\x1C')[0];

            console.log(`[Route ${route.name}] ✅ Target Response:`, resText.substring(0, 100) + (resText.length > 100 ? "..." : ""));
            resolve({ success: true, response: resText });
          });
          
          client.on('error', (err) => {
            if (timeoutTriggered) return;
            clearTimeout(timer);
            console.error(`[Route ${route.name}] ❌ TCP Target Error:`, err.message);
            resolve({ success: false, error: err.message });
          });
        });

        if (!tcpResult.success) {
           status = 'ERROR'; errorMsg = tcpResult.error;
        } else {
           targetResponse = tcpResult.response || '';
        }
        return tcpResult;
      } else if (target.protocol === 'database') {
        console.log("Mock inserting into DB:", transformedData);
        targetResponse = 'Inserted into DB';
        return { success: true, response: 'Inserted into DB' };
      }

      status = 'ERROR'; errorMsg = 'Unsupported target protocol';
      return { success: false, error: errorMsg };
    } catch (error) {
      status = 'ERROR'; errorMsg = error.message;
      console.error(`Error executing route ${routeId}:`, error);
      return { success: false, error: error.message };
    } finally {
      // Save log to database
      try {
        const sourceProto = route.source ? route.source.protocol : 'unknown';
        const targetProto = route.target ? route.target.protocol : 'unknown';
        const inDataStr = typeof inputData === 'string' ? inputData : JSON.stringify(inputData);
        const outDataStr = typeof transformedData === 'string' ? transformedData : JSON.stringify(transformedData);
        
        await this.db.dbRun(
          `INSERT INTO message_logs (id, route_id, source_protocol, target_protocol, inbound_data, outbound_data, response_data, status, error_message, created_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
          [crypto.randomUUID(), routeId, sourceProto, targetProto, inDataStr, outDataStr, targetResponse, status, errorMsg, new Date().toISOString()]
        );
      } catch (e) {
        console.error("Failed to save message log:", e.message);
      }
    }
  }
}
