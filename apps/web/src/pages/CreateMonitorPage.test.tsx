import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { CreateMonitorPage } from './CreateMonitorPage';
import type { MonitorClient } from '../entities/monitor';
const client: MonitorClient = {
  createMonitor: vi.fn(),
  listMonitors: vi.fn().mockResolvedValue({
    kind: 'success',
    data: { items: [], nextCursor: null },
  }),
  getMonitor: vi.fn(),
  getAvailability: vi.fn(),
  getLatestResult: vi.fn(),
};
it('opens a known Monitor by ID without creating it', async () => {
  const navigate = vi.fn();
  render(<CreateMonitorPage client={client} onNavigate={navigate} />);
  const id = 'aa29443e-c597-4e7b-9202-a4762e0e04c0';
  await userEvent.type(screen.getByLabelText('Monitor ID'), id);
  await userEvent.click(screen.getByRole('button', { name: 'Open monitor' }));
  await waitFor(() => expect(navigate).toHaveBeenCalledWith('/monitors/' + id));
  expect(client.createMonitor).not.toHaveBeenCalled();
});
it('provides an input error for invalid Monitor ID', async () => {
  render(<CreateMonitorPage client={client} onNavigate={vi.fn()} />);
  await userEvent.click(screen.getByRole('button', { name: 'Open monitor' }));
  expect(screen.getByRole('alert')).toHaveTextContent('valid Monitor ID');
  expect(screen.getByLabelText('Monitor ID')).toHaveFocus();
});

it('preserves creation and UUID reopen when inventory fails', async () => {
  const id = '018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2',
    navigate = vi.fn();
  const api: MonitorClient = {
    ...client,
    listMonitors: vi.fn().mockResolvedValue({
      kind: 'error',
      error: {
        kind: 'http',
        status: 500,
        message: 'The inventory read request failed.',
      },
    }),
    createMonitor: vi.fn().mockResolvedValue({
      kind: 'created',
      monitor: {
        id,
        targetUrl: 'http://web/',
        createdAt: '2026-10-05T00:00:00Z',
      },
    }),
  };
  render(<CreateMonitorPage client={api} onNavigate={navigate} />);
  expect(await screen.findByRole('alert')).toHaveTextContent('inventory read');
  await userEvent.type(screen.getByLabelText('Target URL'), 'http://web/');
  await userEvent.keyboard('{Enter}');
  await waitFor(() => expect(navigate).toHaveBeenCalledWith('/monitors/' + id));
  await userEvent.type(screen.getByLabelText('Monitor ID'), id);
  await userEvent.click(screen.getByRole('button', { name: 'Open monitor' }));
  expect(navigate).toHaveBeenCalledTimes(2);
  expect(api.createMonitor).toHaveBeenCalledTimes(1);
});
