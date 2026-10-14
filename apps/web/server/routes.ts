import { validateInventoryQuery } from './inventory-query.js';
export type RouteDecision =
  | { kind: 'forward'; path: string; method: 'GET' | 'POST' | 'PUT' }
  | { kind: 'reject'; status: number; allow?: string }
  | { kind: 'not_api' };
export function matchApiRoute(
  rawTarget: string,
  method: string,
): RouteDecision {
  const queryAt = rawTarget.indexOf('?');
  const rawPath = queryAt === -1 ? rawTarget : rawTarget.slice(0, queryAt);
  if (
    !rawTarget.startsWith('/') ||
    /[%\\#]/.test(rawTarget) ||
    rawPath.includes('//') ||
    rawPath.split('/').some((segment) => segment === '.' || segment === '..')
  )
    return { kind: 'reject', status: 400 };
  if (
    queryAt !== -1 &&
    (rawPath !== '/api/monitors' ||
      method !== 'GET' ||
      queryAt === rawTarget.length - 1 ||
      !validateInventoryQuery(rawTarget.slice(queryAt + 1)))
  )
    return { kind: 'reject', status: 400 };
  if (rawPath === '/api/monitors') {
    if (method !== 'GET' && method !== 'POST')
      return { kind: 'reject', status: 405, allow: 'GET, POST' };
    return { kind: 'forward', path: rawTarget.slice(4), method };
  }
  if (rawTarget !== '/api' && !rawTarget.startsWith('/api/'))
    return { kind: 'not_api' };
  if (rawTarget.endsWith('/')) return { kind: 'reject', status: 400 };
  if (/^\/api\/monitors\/[A-Za-z0-9-]+\/scheduling$/.test(rawTarget)) {
    if (method !== 'GET' && method !== 'PUT')
      return { kind: 'reject', status: 405, allow: 'GET, PUT' };
    return { kind: 'forward', path: rawTarget.slice(4), method };
  }
  const allowed =
    rawTarget === '/api/monitors'
      ? 'POST'
      : /^\/api\/monitors\/[A-Za-z0-9-]+(?:\/(?:latest-result|availability))?$/.test(
            rawTarget,
          )
        ? 'GET'
        : null;
  if (!allowed) return { kind: 'reject', status: 404 };
  if (method !== allowed)
    return { kind: 'reject', status: 405, allow: allowed };
  return { kind: 'forward', path: rawTarget.slice(4), method: allowed };
}
