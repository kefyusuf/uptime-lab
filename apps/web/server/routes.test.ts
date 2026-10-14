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
    kind: 'reject',
    status: 404,
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
