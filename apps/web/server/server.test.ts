import { createServer, request, type Server } from 'node:http';
import { once } from 'node:events';
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { afterEach, beforeEach, expect, it } from 'vitest';
import { loadAssets } from './assets.js';
import { loadWebConfig } from './config.js';
import { createWebServer, shutdownWebServer } from './server.js';
let root: string, server: Server, port: number;
beforeEach(async () => {
  root = mkdtempSync(join(tmpdir(), 'uptime-server-'));
  mkdirSync(join(root, '.vite'));
  mkdirSync(join(root, 'assets'));
  writeFileSync(join(root, 'index.html'), '<h1>Web</h1>');
  writeFileSync(join(root, 'assets/main.js'), 'console.log("built");');
  writeFileSync(
    join(root, '.vite/manifest.json'),
    JSON.stringify({ 'index.html': { file: 'assets/main.js', isEntry: true } }),
  );
  server = createWebServer(loadWebConfig({}), loadAssets(root), () => {});
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw Error('address');
  port = address.port;
});
afterEach(async () => {
  if (server) {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }
  if (root) rmSync(root, { recursive: true, force: true });
});
async function call(path: string, host = '127.0.0.1:4173') {
  return new Promise<{
    status: number;
    body: string;
    headers: Record<string, unknown>;
  }>((resolve, reject) => {
    const req = request(
      { hostname: '127.0.0.1', port, path, headers: { Host: host } },
      (res) => {
        let body = '';
        res.on('data', (chunk) => {
          body += chunk;
        });
        res.on('end', () =>
          resolve({ status: res.statusCode || 0, body, headers: res.headers }),
        );
      },
    );
    req.on('error', reject);
    req.end();
  });
}
it('serves recognized pages and manifest assets with security headers', async () => {
  const html = await call('/monitors/id');
  expect(html.status).toBe(200);
  expect(html.body).toContain('<h1>Web');
  expect(html.headers).toMatchObject({
    'cache-control': 'no-store',
    'x-content-type-options': 'nosniff',
    'referrer-policy': 'no-referrer',
  });
  const policy = String(html.headers['content-security-policy']);
  expect(policy).toContain("script-src 'self'");
  expect(policy).toContain("object-src 'none'");
  expect(policy).toContain("frame-ancestors 'none'");
  expect(policy).not.toContain('unsafe-inline');
  const asset = await call('/assets/main.js');
  expect(asset.status).toBe(200);
  expect(asset.headers['cache-control']).toContain('immutable');
});
it.each([
  '/api/internal/checks/claim',
  '/internal/checks/claim',
  '/readyz',
  '/livez',
  '/assets/missing.js',
  '/secret',
  '/assets/%2e%2e/secret',
  '/assets/../secret',
])('does not use shell fallback for %s', async (path) =>
  expect((await call(path)).status).toBeGreaterThanOrEqual(400),
);
it('allows only process/assets health outside browser Host rule', async () => {
  expect(await call('/healthz', 'web:8080')).toMatchObject({
    status: 200,
    body: 'ok\n',
  });
  expect((await call('/', 'web:8080')).status).toBe(403);
  rmSync(join(root, 'assets/main.js'));
  expect((await call('/healthz', 'web:8080')).status).toBe(503);
});
it('rejects oversized headers with no-store', async () => {
  const result = await new Promise<{ status: number; cache: unknown }>(
    (resolve, reject) => {
      const req = request(
        {
          hostname: '127.0.0.1',
          port,
          path: '/',
          headers: { Host: '127.0.0.1:4173', 'X-Large': 'x'.repeat(16384) },
        },
        (res) => {
          res.resume();
          resolve({
            status: res.statusCode || 0,
            cache: res.headers['cache-control'],
          });
        },
      );
      req.on('error', reject);
      req.end();
    },
  );
  expect(result).toEqual({ status: 431, cache: 'no-store' });
});
it('closes active requests after bounded shutdown grace', async () => {
  const hanging = createServer(() => {});
  hanging.listen(0, '127.0.0.1');
  await once(hanging, 'listening');
  const address = hanging.address();
  if (!address || typeof address === 'string') throw Error('address');
  const req = request({ hostname: '127.0.0.1', port: address.port });
  req.on('error', () => {});
  req.end();
  await once(hanging, 'request');
  await shutdownWebServer(hanging, 20);
  expect(hanging.listening).toBe(false);
  req.destroy();
});
