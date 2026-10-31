import { createServer, type Server } from 'node:http';
import type { WebConfig } from './config.js';
import { serveAsset, type AssetIndex } from './assets.js';
import { checkBrowserBoundary, createGateway } from './gateway.js';
import { gatewayError } from './proxy.js';
export interface WebLog {
  category: string;
  method: string;
  status: number;
  durationMs: number;
}
type LogSink = (entry: WebLog) => void;
const csp =
  "default-src 'self'; script-src 'self'; style-src 'self'; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'";
export function createWebServer(
  config: WebConfig,
  assets: AssetIndex,
  log: LogSink = (entry) => console.info(JSON.stringify(entry)),
): Server {
  const gateway = createGateway(config);
  const server = createServer(
    {
      maxHeaderSize: config.maxHeaderBytes,
      headersTimeout: config.headerTimeoutMs,
      requestTimeout: config.requestTimeoutMs,
      connectionsCheckingInterval: Math.min(250, config.headerTimeoutMs),
    },
    (req, res) => {
      const started = performance.now();
      const raw = req.url || '';
      const category =
        raw === '/healthz'
          ? 'health'
          : raw.startsWith('/api/')
            ? 'api'
            : 'asset';
      res.once('finish', () =>
        log({
          category,
          method: ['GET', 'POST', 'HEAD', 'OPTIONS'].includes(req.method || '')
            ? req.method!
            : 'OTHER',
          status: res.statusCode,
          durationMs: Math.max(0, Math.round(performance.now() - started)),
        }),
      );
      res.setHeader('X-Content-Type-Options', 'nosniff');
      res.setHeader('Referrer-Policy', 'no-referrer');
      res.setHeader('Content-Security-Policy', csp);
      if (raw === '/healthz') {
        if (req.method !== 'GET' && req.method !== 'HEAD') {
          gatewayError(res, 405, 'GET, HEAD');
          return;
        }
        const ready = assets.ready();
        res.writeHead(ready ? 200 : 503, {
          'Content-Type': 'text/plain; charset=utf-8',
          'Cache-Control': 'no-store',
        });
        res.end(ready ? 'ok\n' : 'unavailable\n');
        return;
      }
      const boundary = checkBrowserBoundary(req, config, false);
      if (boundary) {
        gatewayError(res, boundary.status);
        return;
      }
      if (raw === '/api' || raw.startsWith('/api/')) {
        gateway(req, res);
        return;
      }
      if (!assets.ready()) {
        gatewayError(res, 503);
        return;
      }
      if (!serveAsset(req, res, assets)) gatewayError(res, 404);
    },
  );
  server.on('clientError', (error, socket) => {
    const status =
      (error as NodeJS.ErrnoException).code === 'HPE_HEADER_OVERFLOW'
        ? 431
        : 400;
    const body = JSON.stringify({ title: 'Invalid request.', status });
    if (socket.writable)
      socket.end(
        'HTTP/1.1 ' +
          status +
          ' Error\r\nConnection: close\r\nContent-Type: application/problem+json\r\nCache-Control: no-store\r\nContent-Length: ' +
          Buffer.byteLength(body) +
          '\r\n\r\n' +
          body,
      );
    else socket.destroy();
  });
  return server;
}
export async function shutdownWebServer(
  server: Server,
  graceMs = 5000,
): Promise<void> {
  if (!server.listening) return;
  await new Promise<void>((resolve, reject) => {
    const timer = setTimeout(() => server.closeAllConnections(), graceMs);
    server.close((error) => {
      clearTimeout(timer);
      if (error) reject(error);
      else resolve();
    });
  });
}
