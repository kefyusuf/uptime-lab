import {
  createServer,
  request,
  type Server,
  type IncomingHttpHeaders,
} from 'node:http';
import { once } from 'node:events';
import { afterEach, beforeEach, describe, expect, it } from 'vitest';
import { loadWebConfig, type WebConfig } from './config.js';
import { createGateway } from './gateway.js';

let upstream: Server, gateway: Server, port: number, config: WebConfig;
let captured: { path?: string; headers: IncomingHttpHeaders; body: string }[];
let status: number,
  body: string,
  responseHeaders: Record<string, string>,
  hang: boolean;
async function listen(server: Server): Promise<number> {
  server.listen(0, '127.0.0.1');
  await once(server, 'listening');
  const address = server.address();
  if (!address || typeof address === 'string') throw Error('address');
  return address.port;
}
function call(
  path: string,
  method = 'GET',
  headers: Record<string, string> | string[] = {},
  payload?: string,
): Promise<{ status: number; headers: IncomingHttpHeaders; body: string }> {
  return new Promise((resolve, reject) => {
    const defaults = { Host: '127.0.0.1:4173' };
    const req = request(
      {
        hostname: '127.0.0.1',
        port,
        path,
        method,
        headers: Array.isArray(headers) ? headers : { ...defaults, ...headers },
      },
      (res) => {
        let body = '';
        res.on('data', (chunk) => {
          body += chunk;
        });
        res.on('end', () =>
          resolve({ status: res.statusCode || 0, headers: res.headers, body }),
        );
      },
    );
    req.on('error', reject);
    req.end(payload);
  });
}
beforeEach(async () => {
  captured = [];
  status = 200;
  body = '{"ok":true}';
  responseHeaders = { 'Content-Type': 'application/json' };
  hang = false;
  upstream = createServer((req, res) => {
    let input = '';
    req.on('data', (chunk) => {
      input += chunk;
    });
    req.on('end', () => {
      captured.push({ path: req.url, headers: req.headers, body: input });
      if (!hang) {
        res.writeHead(status, responseHeaders);
        res.end(body);
      }
    });
  });
  const upstreamPort = await listen(upstream);
  config = {
    ...loadWebConfig({}),
    upstreamOrigin: 'http://127.0.0.1:' + upstreamPort,
    upstreamTimeoutMs: 100,
    requestTimeoutMs: 100,
  };
  gateway = createServer(
    { maxHeaderSize: config.maxHeaderBytes },
    createGateway(config),
  );
  port = await listen(gateway);
});
afterEach(async () => {
  for (const server of [gateway, upstream]) {
    if (server) {
      server.closeAllConnections();
      await new Promise<void>((resolve) => server.close(() => resolve()));
    }
  }
});
describe('actual HTTP gateway', () => {
  it('rejects unexpected GET request bodies before upstream work', async () => {
    const result = await call(
      '/api/monitors/id',
      'GET',
      { 'Content-Length': '2' },
      '{}',
    );
    expect(result.status).toBe(400);
    expect(captured).toHaveLength(0);
  });
  it('applies Host checks even on unknown API resources', async () => {
    expect(
      (await call('/api/internal', 'GET', { Host: 'evil.example' })).status,
    ).toBe(403);
  });
  it('forwards all four operations and preserves bytes', async () => {
    for (const path of [
      '/api/monitors/id',
      '/api/monitors/id/latest-result',
      '/api/monitors/id/availability',
    ])
      expect((await call(path)).body).toBe(body);
    status = 201;
    responseHeaders.Location = '/monitors/aa29443e-c597-4e7b-9202-a4762e0e04c0';
    const result = await call(
      '/api/monitors',
      'POST',
      { Origin: 'http://127.0.0.1:4173', 'Content-Type': 'application/json' },
      '{"targetUrl":"http://web/"}',
    );
    expect(result.status).toBe(201);
    expect(result.headers.location).toBe(
      '/api/monitors/aa29443e-c597-4e7b-9202-a4762e0e04c0',
    );
    expect(captured.map((value) => value.path)).toEqual([
      '/monitors/id',
      '/monitors/id/latest-result',
      '/monitors/id/availability',
      '/monitors',
    ]);
  });
  it.each([
    ['/api/internal/checks/claim', 'POST', {}, 404],
    ['/api/monitors/%2e%2e', 'GET', {}, 400],
    ['/api/monitors/id', 'HEAD', {}, 405],
    ['/api/monitors/id', 'GET', { Host: 'evil.example' }, 403],
    ['/api/monitors/id', 'GET', { Origin: 'null' }, 403],
    ['/api/monitors/id', 'GET', { 'Sec-Fetch-Site': 'cross-site' }, 403],
    ['/api/monitors', 'POST', { 'Content-Type': 'application/json' }, 403],
    [
      '/api/monitors',
      'POST',
      { Origin: 'http://evil.example', 'Content-Type': 'application/json' },
      403,
    ],
    [
      '/api/monitors',
      'POST',
      {
        Origin: 'http://127.0.0.1:4173',
        'Content-Type': 'application/x-www-form-urlencoded',
      },
      415,
    ],
  ] as const)(
    'rejects %s %s before forwarding',
    async (path, method, headers, status) => {
      const result = await call(path, method, headers);
      expect(result.status).toBe(status);
      expect(result.headers['cache-control']).toBe('no-store');
      expect(captured).toHaveLength(0);
    },
  );
  it('rejects duplicate Host headers', async () => {
    const result = await call('/api/monitors/id', 'GET', [
      'Host',
      '127.0.0.1:4173',
      'Host',
      'evil.example',
    ]);
    expect(result.status).toBe(403);
    expect(captured).toHaveLength(0);
  });
  it('filters sensitive headers both directions', async () => {
    responseHeaders['Set-Cookie'] = 'secret=x';
    responseHeaders['X-Internal'] = 'api:8080';
    const result = await call('/api/monitors/id', 'GET', {
      Cookie: 'secret=x',
      Authorization: 'secret',
      'X-Forwarded-Host': 'evil',
    });
    expect(captured).toHaveLength(1);
    expect(captured[0].headers.cookie).toBeUndefined();
    expect(captured[0].headers.authorization).toBeUndefined();
    expect(captured[0].headers['x-forwarded-host']).toBeUndefined();
    expect(result.headers['set-cookie']).toBeUndefined();
    expect(result.headers['x-internal']).toBeUndefined();
  });
  it('preserves204 and problem status with no-store', async () => {
    status = 204;
    body = '';
    expect(await call('/api/monitors/id/latest-result')).toMatchObject({
      status: 204,
      body: '',
      headers: { 'cache-control': 'no-store' },
    });
    status = 400;
    body = '{"title":"Invalid monitor ID"}';
    responseHeaders['Content-Type'] = 'application/problem+json';
    expect(await call('/api/monitors/invalid')).toMatchObject({
      status: 400,
      body,
      headers: {
        'content-type': 'application/problem+json',
        'cache-control': 'no-store',
      },
    });
  });
  it.each(['/internal/checks/claim', 'http://evil.example/path'])(
    'rejects invalid creation Location %s',
    async (location) => {
      status = 201;
      responseHeaders.Location = location;
      const result = await call(
        '/api/monitors',
        'POST',
        { Origin: 'http://127.0.0.1:4173', 'Content-Type': 'application/json' },
        '{}',
      );
      expect(result.status).toBe(502);
      expect(result.body).not.toContain(location);
    },
  );
  it('does not follow upstream redirect', async () => {
    status = 302;
    responseHeaders.Location = 'http://evil.example/path';
    expect((await call('/api/monitors/id')).status).toBe(502);
    expect(captured).toHaveLength(1);
  });
  it('bounds chunked request without waiting for end', async () => {
    const result = await new Promise<number>((resolve, reject) => {
      const req = request(
        {
          hostname: '127.0.0.1',
          port,
          path: '/api/monitors',
          method: 'POST',
          headers: {
            Host: '127.0.0.1:4173',
            Origin: 'http://127.0.0.1:4173',
            'Content-Type': 'application/json',
            'Transfer-Encoding': 'chunked',
          },
        },
        (res) => {
          resolve(res.statusCode || 0);
          res.resume();
          req.destroy();
        },
      );
      req.on('error', reject);
      req.write('x'.repeat(65537));
    });
    expect(result).toBe(413);
    expect(captured).toHaveLength(0);
  });
  it('bounds upstream body', async () => {
    body = 'x'.repeat(262145);
    expect((await call('/api/monitors/id')).status).toBe(502);
  });
  it('terminates slow upstream with sanitized504', async () => {
    hang = true;
    const result = await call('/api/monitors/id');
    expect(result.status).toBe(504);
    expect(result.body).not.toContain('127.0.0.1');
  });
  it('aborts upstream when browser disconnects', async () => {
    hang = true;
    const disconnected = new Promise<void>((resolve) =>
      upstream.once('request', (_req, res) =>
        res.once('close', () => resolve()),
      ),
    );
    const req = request({
      hostname: '127.0.0.1',
      port,
      path: '/api/monitors/id',
      headers: { Host: '127.0.0.1:4173' },
    });
    req.on('error', () => {});
    req.end();
    await new Promise<void>((resolve) =>
      upstream.once('request', () => resolve()),
    );
    req.destroy();
    await disconnected;
    expect(captured).toHaveLength(1);
  });
});
