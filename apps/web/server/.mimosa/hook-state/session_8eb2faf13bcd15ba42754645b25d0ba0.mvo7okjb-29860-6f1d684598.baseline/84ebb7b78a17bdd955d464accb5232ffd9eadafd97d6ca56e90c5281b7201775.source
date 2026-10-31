import {
  Agent,
  request,
  type IncomingMessage,
  type ServerResponse,
} from 'node:http';
import type { WebConfig } from './config.js';
import type { RouteDecision } from './routes.js';

export function gatewayError(
  res: ServerResponse,
  status: number,
  allow?: string,
): void {
  if (res.writableEnded || res.destroyed) return;
  res.writeHead(status, {
    'Content-Type': 'application/problem+json',
    'Cache-Control': 'no-store',
    Connection: 'close',
    ...(allow ? { Allow: allow } : {}),
  });
  res.end(
    JSON.stringify({
      title:
        status === 504
          ? 'The API request timed out.'
          : 'The request could not be completed.',
      status,
    }),
  );
}
export function forwardApi(
  req: IncomingMessage,
  res: ServerResponse,
  route: Extract<RouteDecision, { kind: 'forward' }>,
  body: Buffer,
  config: WebConfig,
): void {
  const upstreamOrigin = new URL(config.upstreamOrigin);
  const agent = new Agent({ keepAlive: false });
  let completed = false;
  const upstream = request({
    protocol: upstreamOrigin.protocol,
    hostname: upstreamOrigin.hostname,
    port: upstreamOrigin.port,
    path: route.path,
    method: route.method,
    agent,
    headers: {
      Accept: 'application/json',
      ...(route.method === 'POST'
        ? { 'Content-Type': 'application/json', 'Content-Length': body.length }
        : {}),
    },
  });
  const cleanup = () => {
    clearTimeout(timer);
    res.off('close', disconnect);
    req.off('aborted', disconnect);
    agent.destroy();
  };
  const fail = (status: number) => {
    if (completed) return;
    completed = true;
    gatewayError(res, status);
    upstream.destroy();
    cleanup();
  };
  const disconnect = () => {
    if (!res.writableFinished) {
      completed = true;
      upstream.destroy();
      cleanup();
    }
  };
  const timer = setTimeout(() => fail(504), config.upstreamTimeoutMs);
  req.once('aborted', disconnect);
  res.once('close', disconnect);
  upstream.on('error', () => fail(502));
  upstream.on('response', (source) => {
    const status = source.statusCode || 502;
    if (status >= 300 && status < 400) {
      source.destroy();
      fail(502);
      return;
    }
    const location = source.headers.location;
    if (
      location &&
      (route.method !== 'POST' ||
        status !== 201 ||
        !/^\/monitors\/[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
          location,
        ))
    ) {
      source.destroy();
      fail(502);
      return;
    }
    const parts: Buffer[] = [];
    let size = 0;
    source.on('data', (chunk: Buffer) => {
      size += chunk.length;
      if (size > config.maxResponseBytes) {
        source.destroy();
        fail(502);
      } else parts.push(chunk);
    });
    source.on('error', () => fail(502));
    source.on('aborted', () => fail(502));
    source.on('end', () => {
      if (completed) return;
      completed = true;
      const headers: Record<string, string> = { 'Cache-Control': 'no-store' };
      if (source.headers['content-type'])
        headers['Content-Type'] = source.headers['content-type'];
      if (source.headers.allow) headers.Allow = source.headers.allow;
      if (location) headers.Location = '/api' + location;
      res.writeHead(status, headers);
      res.end(Buffer.concat(parts));
      cleanup();
    });
  });
  upstream.end(body);
}
