// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest';
import { navigate, parsePage } from './router';
afterEach(() => window.history.replaceState(null, '', '/'));
it('recognizes only creation and detail routes', () => {
  expect(parsePage('/')).toEqual({ kind: 'create' });
  expect(parsePage('/monitors/ABC-id')).toEqual({
    kind: 'detail',
    id: 'ABC-id',
  });
  for (const path of [
    '/monitors',
    '/monitors/id/history',
    '//evil.example',
    '/monitors/%2e%2e',
  ])
    expect(parsePage(path)).toEqual({ kind: 'missing' });
});
it('updates browser history and notifies popstate subscribers', () => {
  const listener = vi.fn();
  window.addEventListener('popstate', listener);
  try {
    navigate('/monitors/id');
    expect(window.location.pathname).toBe('/monitors/id');
    expect(listener).toHaveBeenCalledOnce();
  } finally {
    window.removeEventListener('popstate', listener);
  }
});
it('refuses external navigation', () =>
  expect(() => navigate('https://evil.example')).toThrow());
