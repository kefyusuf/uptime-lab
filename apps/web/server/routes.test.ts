import { expect, it } from 'vitest';
import { matchApiRoute } from './routes.js';
it.each([
  ['/api/monitors', 'POST', '/monitors'],
  ['/api/monitors/ABC-invalid', 'GET', '/monitors/ABC-invalid'],
  ['/api/monitors/id/latest-result', 'GET', '/monitors/id/latest-result'],
  ['/api/monitors/id/availability', 'GET', '/monitors/id/availability'],
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
  expect(matchApiRoute('/api/monitors', 'GET')).toEqual({
    kind: 'reject',
    status: 405,
    allow: 'POST',
  });
});
