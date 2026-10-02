export type RouteDecision =
  | { kind: 'forward'; path: string; method: 'GET' | 'POST' }
  | { kind: 'reject'; status: number; allow?: string }
  | { kind: 'not_api' };
export function matchApiRoute(
  rawTarget: string,
  method: string,
): RouteDecision {
  if (
    !rawTarget.startsWith('/') ||
    /[%\\?#]/.test(rawTarget) ||
    rawTarget.includes('//') ||
    rawTarget.split('/').some((segment) => segment === '.' || segment === '..')
  )
    return { kind: 'reject', status: 400 };
  if (rawTarget !== '/api' && !rawTarget.startsWith('/api/'))
    return { kind: 'not_api' };
  if (rawTarget.endsWith('/')) return { kind: 'reject', status: 400 };
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
