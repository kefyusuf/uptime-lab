import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import { CreateMonitorPage } from './CreateMonitorPage';
import type { MonitorClient } from '../entities/monitor';
const client: MonitorClient = {
  createMonitor: vi.fn(),
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
