import type { IncomingMessage, RequestListener } from 'node:http';
import type { WebConfig } from './config.js';
import { matchApiRoute } from './routes.js';
import { forwardApi, gatewayError } from './proxy.js';
export function checkBrowserBoundary(
  req: IncomingMessage,
  config: WebConfig,
  requireOrigin: boolean,
): { status: 403 } | null {
  let hosts = 0,
    origins = 0;
  for (let index = 0; index < req.rawHeaders.length; index += 2) {
    const name = req.rawHeaders[index].toLowerCase();
    if (name === 'host') hosts++;
    if (name === 'origin') origins++;
  }
  const origin = 'http://' + req.headers.host;
  if (
    hosts !== 1 ||
    !config.allowedOrigins.includes(origin) ||
    origins > 1 ||
    req.headers['sec-fetch-site'] === 'cross-site'
  )
    return { status: 403 };
  if (req.headers.origin !== undefined && req.headers.origin !== origin)
    return { status: 403 };
  if (requireOrigin && (origins !== 1 || req.headers.origin !== origin))
    return { status: 403 };
  return null;
}
export function createGateway(config: WebConfig): RequestListener {
  return (req, res) => {
    const initialBoundary = checkBrowserBoundary(req, config, false);
    if (initialBoundary) {
      gatewayError(res, initialBoundary.status);
      return;
    }
    const route = matchApiRoute(req.url || '', req.method || '');
    if (route.kind === 'not_api') {
      gatewayError(res, 404);
      return;
    }
    if (route.kind === 'reject') {
      gatewayError(res, route.status, route.allow);
      return;
    }
    const boundary = checkBrowserBoundary(req, config, route.method === 'POST');
    if (boundary) {
      gatewayError(res, boundary.status);
      return;
    }
    if (
      route.method === 'POST' &&
      !/^application\/json(?:\s*;\s*charset=[A-Za-z0-9-]+)?$/i.test(
        req.headers['content-type'] || '',
      )
    ) {
      gatewayError(res, 415);
      return;
    }
    if (Number(req.headers['content-length']) > config.maxRequestBytes) {
      gatewayError(res, 413);
      req.resume();
      return;
    }
    if (route.method === 'GET') {
      if (
        req.headers['transfer-encoding'] ||
        Number(req.headers['content-length']) > 0
      ) {
        gatewayError(res, 400);
        req.resume();
        return;
      }
      forwardApi(req, res, route, Buffer.alloc(0), config);
      return;
    }
    let size = 0,
      done = false;
    const parts: Buffer[] = [];
    const timeout = setTimeout(() => {
      if (!done) {
        done = true;
        gatewayError(res, 408);
        req.resume();
      }
    }, config.requestTimeoutMs);
    const clean = () => clearTimeout(timeout);
    req.once('aborted', clean);
    res.once('close', clean);
    req.on('error', () => {
      clean();
      if (!done) {
        done = true;
        gatewayError(res, 400);
      }
    });
    req.on('data', (chunk: Buffer) => {
      if (done) return;
      size += chunk.length;
      if (size > config.maxRequestBytes) {
        done = true;
        clean();
        gatewayError(res, 413);
        req.resume();
      } else parts.push(chunk);
    });
    req.on('end', () => {
      clean();
      if (!done) {
        done = true;
        forwardApi(req, res, route, Buffer.concat(parts), config);
      }
    });
  };
}
