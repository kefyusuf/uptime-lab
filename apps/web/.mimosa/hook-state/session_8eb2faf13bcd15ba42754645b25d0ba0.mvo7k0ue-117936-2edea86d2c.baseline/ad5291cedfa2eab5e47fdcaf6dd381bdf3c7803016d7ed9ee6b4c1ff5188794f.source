import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, expect, it, vi } from 'vitest';
import type { MonitorClient } from '../entities/monitor';
import { App } from './App';
const id = 'aa29443e-c597-4e7b-9202-a4762e0e04c0';
const monitor = {
  id,
  targetUrl: 'https://example.com/',
  createdAt: '2026-10-02T12:00:00Z',
};
const client: MonitorClient = {
  createMonitor: vi.fn().mockResolvedValue({ kind: 'created', monitor }),
  listMonitors: vi.fn().mockResolvedValue({
    kind: 'success',
    data: { items: [], nextCursor: null },
  }),
  getMonitor: vi.fn().mockResolvedValue({ kind: 'success', data: monitor }),
  getAvailability: vi.fn().mockResolvedValue({
    kind: 'success',
    data: {
      status: 'unknown',
      reason: 'no_result',
      evaluatedAt: '2026-10-02T12:00:00Z',
    },
  }),
  getLatestResult: vi.fn().mockResolvedValue({ kind: 'success', data: null }),
};
afterEach(() => window.history.replaceState(null, '', '/'));
it('supports keyboard creation and detail navigation', async () => {
  window.history.replaceState(null, '', '/');
  render(<App client={client} />);
  await userEvent.type(
    screen.getByLabelText('Target URL'),
    'https://example.com/{Enter}',
  );
  expect(
    await screen.findByRole('heading', { name: 'Monitor detail' }),
  ).toBeInTheDocument();
  expect(window.location.pathname).toBe('/monitors/' + id);
});
it('reacts to native back and forward route events', async () => {
  window.history.replaceState(null, '', '/');
  window.history.pushState(null, '', '/monitors/' + id);
  render(<App client={client} />);
  expect(
    screen.getByRole('heading', { name: 'Monitor detail' }),
  ).toBeInTheDocument();
  window.history.back();
  expect(
    await screen.findByRole('heading', { name: 'Create monitor' }),
  ).toBeInTheDocument();
  window.history.forward();
  expect(
    await screen.findByRole('heading', { name: 'Monitor detail' }),
  ).toBeInTheDocument();
});
