import { fireEvent, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import type { MonitorClient } from '../../entities/monitor';
import { MonitorInventory } from './MonitorInventory';
const row = {
  id: '018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2',
  targetUrl: 'http://web/?value=<script>&long=' + 'x'.repeat(500),
  createdAt: '2026-10-05T00:00:00.123456Z',
};
function api(): MonitorClient {
  return {
    listMonitors: vi.fn(),
    createMonitor: vi.fn(),
    getMonitor: vi.fn(),
    getAvailability: vi.fn(),
    getLatestResult: vi.fn(),
  };
}
it('distinguishes first-page empty from cursor-page empty', async () => {
  const client = api();
  client.listMonitors = vi
    .fn()
    .mockResolvedValueOnce({
      kind: 'success',
      data: { items: [], nextCursor: null },
    })
    .mockResolvedValueOnce({
      kind: 'success',
      data: { items: [row], nextCursor: 'A'.repeat(88) },
    })
    .mockResolvedValueOnce({
      kind: 'success',
      data: { items: [], nextCursor: null },
    });
  render(<MonitorInventory client={client} onNavigate={vi.fn()} />);
  expect(await screen.findByText('No monitors registered yet.')).toBeVisible();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh list' }));
  await userEvent.click(
    await screen.findByRole('button', { name: 'Next page' }),
  );
  expect(
    await screen.findByText(
      'No more monitors on this page. Refresh the list to start again.',
    ),
  ).toBeVisible();
  expect(
    screen.queryByText('No monitors registered yet.'),
  ).not.toBeInTheDocument();
});
it('preserves exact text and local keyboard/modified-click links', async () => {
  const client = api(),
    navigate = vi.fn();
  client.listMonitors = vi.fn().mockResolvedValue({
    kind: 'success',
    data: { items: [row], nextCursor: null },
  });
  render(<MonitorInventory client={client} onNavigate={navigate} />);
  expect(await screen.findByText(row.targetUrl)).toBeVisible();
  const link = screen.getByRole('link', { name: 'Open monitor ' + row.id });
  expect(link).toHaveAttribute('href', '/monitors/' + row.id);
  fireEvent.click(link, { ctrlKey: true });
  expect(navigate).not.toHaveBeenCalled();
  link.focus();
  await userEvent.keyboard('{Enter}');
  expect(navigate).toHaveBeenCalledWith('/monitors/' + row.id);
  expect(client.getMonitor).not.toHaveBeenCalled();
  expect(document.querySelector('script')).toBeNull();
});
it('shows a read error instead of empty and permits Retry', async () => {
  const client = api();
  client.listMonitors = vi.fn().mockResolvedValue({
    kind: 'error',
    error: {
      kind: 'http',
      status: 500,
      message: 'A registered monitor exceeds the inventory response limit.',
    },
  });
  render(<MonitorInventory client={client} onNavigate={vi.fn()} />);
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'A registered monitor exceeds',
  );
  expect(
    screen.queryByText('No monitors registered yet.'),
  ).not.toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Retry list' }));
  await waitFor(() => expect(client.listMonitors).toHaveBeenCalledTimes(2));
});
