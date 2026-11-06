import { expect, it } from 'vitest';
import { matchApiRoute } from './routes.js';
it.each([
  ['/api/monitors', 'POST', '/monitors'],
  ['/api/monitors', 'GET', '/monitors'],
  ['/api/monitors?limit=20', 'GET', '/monitors?limit=20'],
  ['/api/monitors/ABC-invalid', 'GET', '/monitors/ABC-invalid'],
  ['/api/monitors/id/latest-result', 'GET', '/monitors/id/latest-result'],
  ['/api/monitors/id/availability', 'GET', '/monitors/id/availability'],
  ['/api/monitors/id/scheduling', 'GET', '/monitors/id/scheduling'],
  ['/api/monitors/id/scheduling', 'PUT', '/monitors/id/scheduling'],
])('maps only public shape %s', (path, method, target) =>
  expect(matchApiRoute(path, method)).toEqual({
    kind: 'forward',
    method,
    path: target,
  }),
);
it.each([
  '/api/monitors/id/',
  '/api//monitors/id',
  '/api/monitors/%2e%2e',
  '/api/monitors/../internal',
  '/api/monitors/id?x=1',
  '/api/monitors/id#fragment',
  'http://localhost/api/monitors',
  '/api\\monitors\\id',
])('rejects noncanonical raw target %s', (path) =>
  expect(matchApiRoute(path, 'GET')).toMatchObject({
    kind: 'reject',
    status: 400,
  }),
);
it('never proxies an internal or unknown API path', () => {
  expect(matchApiRoute('/api/internal/checks/claim', 'POST')).toEqual({
    kind: 'reject',
    status: 404,
  });
  expect(matchApiRoute('/api/monitors/id/history', 'GET')).toEqual({
    kind: 'forward',
    method: 'GET',
    path: '/monitors/id/history',
  });
  expect(matchApiRoute('/internal/checks/claim', 'POST')).toEqual({
    kind: 'not_api',
  });
});
it('rejects HEAD and other methods with exact Allow', () => {
  expect(matchApiRoute('/api/monitors/id/availability', 'HEAD')).toEqual({
    kind: 'reject',
    status: 405,
    allow: 'GET',
  });
  expect(matchApiRoute('/api/monitors', 'HEAD')).toEqual({
    kind: 'reject',
    status: 405,
    allow: 'GET, POST',
  });
});

it.each([
  '/api/monitors?',
  '/api/monitors?limit=020',
  '/api/monitors?limit=20?limit=1',
  '/api/monitors?limit=20&limit=20',
  '/api/monitors?cursor=' + 'A'.repeat(89),
  '/api/monitors?unknown=1',
  '/api/monitors?limit=%32%30',
])('rejects collection query tricks %s', (path) => {
  expect(matchApiRoute(path, 'GET')).toEqual({ kind: 'reject', status: 400 });
});
it('keeps POST and resource query bans', () => {
  expect(matchApiRoute('/api/monitors?limit=20', 'POST')).toEqual({
    kind: 'reject',
    status: 400,
  });
  expect(matchApiRoute('/api/monitors/id?limit=20', 'GET')).toEqual({
    kind: 'reject',
    status: 400,
  });
});

it.each(['HEAD', 'OPTIONS', 'POST', 'PATCH', 'DELETE'])(
  'keeps scheduling verbs closed: %s',
  (method) => {
    expect(matchApiRoute('/api/monitors/id/scheduling', method)).toEqual({
      kind: 'reject',
      status: 405,
      allow: 'GET, PUT',
    });
  },
);
it.each([
  '/api/monitors/id/scheduling?',
  '/api/monitors/id/scheduling?state=paused',
])('rejects scheduling query %s', (target) => {
  expect(matchApiRoute(target, 'PUT')).toEqual({ kind: 'reject', status: 400 });
});
it('maps monitor history beside the other reads', () => {
  expect(matchApiRoute('/api/monitors/id/history', 'GET')).toEqual({
    kind: 'forward',
    method: 'GET',
    path: '/monitors/id/history',
  });
  expect(matchApiRoute('/api/monitors/id/history?limit=7', 'GET')).toEqual({
    kind: 'forward',
    method: 'GET',
    path: '/monitors/id/history?limit=7',
  });
  const cursor = Buffer.from(
    '2|2026-11-01T12:00:00.123456Z|7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675',
  ).toString('base64url');
  expect(
    matchApiRoute('/api/monitors/id/history?cursor=' + cursor, 'GET'),
  ).toEqual({
    kind: 'forward',
    method: 'GET',
    path: '/monitors/id/history?cursor=' + cursor,
  });
  expect(
    matchApiRoute('/api/monitors/id/history?limit=7&cursor=' + cursor, 'GET'),
  ).toEqual({
    kind: 'forward',
    method: 'GET',
    path: '/monitors/id/history?limit=7&cursor=' + cursor,
  });
});
it('keeps history query grammar strict', () => {
  const cursor = 'A'.repeat(88);
  for (const bad of [
    '/api/monitors/id/history?limit=0',
    '/api/monitors/id/history?limit=51',
    '/api/monitors/id/history?limit=abc',
    '/api/monitors/id/history?unknown=1',
    '/api/monitors/id/history?limit=7&limit=7',
    '/api/monitors/id/history?cursor=' + cursor + '&cursor=' + cursor,
    '/api/monitors/id/history?cursor=' + 'B'.repeat(129),
  ]) {
    expect(matchApiRoute(bad, 'GET')).toEqual({ kind: 'reject', status: 400 });
  }
  expect(matchApiRoute('/api/monitors/id?limit=7', 'GET')).toEqual({
    kind: 'reject',
    status: 400,
  });
  expect(
    matchApiRoute('/api/monitors/id/latest-result?limit=7', 'GET'),
  ).toEqual({
    kind: 'reject',
    status: 400,
  });
});
it('keeps history verbs and internal routes closed', () => {
  expect(matchApiRoute('/api/monitors/id/history', 'POST')).toEqual({
    kind: 'reject',
    status: 405,
    allow: 'GET',
  });
  expect(matchApiRoute('/api/internal/checks/claim', 'GET')).toEqual({
    kind: 'reject',
    status: 404,
  });
  expect(matchApiRoute('/api/monitors/id/history/extra', 'GET')).toEqual({
    kind: 'reject',
    status: 404,
  });
});
it('maps monitor deletion beside get', () => {
  expect(matchApiRoute('/api/monitors/id', 'DELETE')).toEqual({
    kind: 'forward',
    method: 'DELETE',
    path: '/monitors/id',
  });
  expect(matchApiRoute('/api/monitors/id', 'GET')).toEqual({
    kind: 'forward',
    method: 'GET',
    path: '/monitors/id',
  });
});
it.each(['HEAD', 'OPTIONS', 'POST', 'PUT', 'PATCH'])(
  'keeps monitor resource verbs closed to get and delete: %s',
  (method) => {
    expect(matchApiRoute('/api/monitors/id', method)).toEqual({
      kind: 'reject',
      status: 405,
      allow: 'GET, DELETE',
    });
  },
);
it('keeps sub-resource deletion closed', () => {
  expect(matchApiRoute('/api/monitors/id/latest-result', 'DELETE')).toEqual({
    kind: 'reject',
    status: 405,
    allow: 'GET',
  });
  expect(matchApiRoute('/api/monitors/id/availability', 'DELETE')).toEqual({
    kind: 'reject',
    status: 405,
    allow: 'GET',
  });
  expect(matchApiRoute('/api/monitors/id/scheduling', 'DELETE')).toEqual({
    kind: 'reject',
    status: 405,
    allow: 'GET, PUT',
  });
});
it.each([
  '/api/monitors/id?',
  '/api/monitors/id?x=1',
  '/api/monitors/id/',
  '/api/monitors/%69d',
])('rejects delete query and alias tricks %s', (path) => {
  expect(matchApiRoute(path, 'DELETE')).toEqual({
    kind: 'reject',
    status: 400,
  });
});
it('keeps internal and unknown routes closed to delete', () => {
  expect(matchApiRoute('/api/internal/checks/claim', 'DELETE')).toEqual({
    kind: 'reject',
    status: 404,
  });
  expect(matchApiRoute('/internal/checks/claim', 'DELETE')).toEqual({
    kind: 'not_api',
  });
  expect(matchApiRoute('/api/monitors/id/history', 'DELETE')).toEqual({
    kind: 'reject',
    status: 405,
    allow: 'GET',
  });
});
