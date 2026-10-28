import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import type { MonitorClient } from '../../entities/monitor';
import { DeleteMonitorControl } from './DeleteMonitorControl';

function client(): MonitorClient {
  return {
    deleteMonitor: vi.fn().mockResolvedValue({ kind: 'deleted' }),
    getMonitor: vi.fn().mockResolvedValue({
      kind: 'error',
      error: { kind: 'http', status: 404, message: 'Monitor not found.' },
    }),
    getScheduling: vi.fn(),
    setScheduling: vi.fn(),
    createMonitor: vi.fn(),
    listMonitors: vi.fn(),
    getAvailability: vi.fn(),
    getLatestResult: vi.fn(),
  };
}
it('requires the permanent-deletion confirmation before deleting', async () => {
  const api = client();
  const onDeleted = vi.fn();
  render(<DeleteMonitorControl id="a" client={api} onDeleted={onDeleted} />);
  expect(
    screen.getByText(
      'Deleting removes the Monitor and all of its check evidence permanently.',
    ),
  ).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Delete monitor' }));
  expect(
    await screen.findByRole('button', { name: 'Delete permanently' }),
  ).toBeEnabled();
  expect(screen.getByRole('button', { name: 'Cancel' })).toBeEnabled();
  expect(api.deleteMonitor).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
  expect(
    screen.getByRole('button', { name: 'Delete monitor' }),
  ).toBeEnabled();
  await userEvent.click(screen.getByRole('button', { name: 'Delete monitor' }));
  await userEvent.click(
    screen.getByRole('button', { name: 'Delete permanently' }),
  );
  await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1));
  expect(api.deleteMonitor).toHaveBeenCalledWith(
    'a',
    expect.any(AbortSignal),
  );
});
it('shows an uncertain outcome with an explicit refresh and no retry', async () => {
  const api = client();
  api.deleteMonitor = vi.fn().mockResolvedValue({
    kind: 'uncertain',
    error: { kind: 'transport', message: 'The deletion outcome is unknown. Refresh to check.' },
  });
  api.getMonitor = vi.fn().mockResolvedValue({
    kind: 'error',
    error: { kind: 'http', status: 404, message: 'Monitor not found.' },
  });
  const onDeleted = vi.fn();
  render(<DeleteMonitorControl id="a" client={api} onDeleted={onDeleted} />);
  await userEvent.click(screen.getByRole('button', { name: 'Delete monitor' }));
  await userEvent.click(
    screen.getByRole('button', { name: 'Delete permanently' }),
  );
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'The deletion outcome is unknown. Refresh to check.',
  );
  expect(api.getMonitor).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole('button', { name: 'Refresh monitor state' }),
  );
  await waitFor(() => expect(onDeleted).toHaveBeenCalledTimes(1));
  expect(api.deleteMonitor).toHaveBeenCalledTimes(1);
});
