import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import type { MonitorClient, SchedulingOutcome } from '../../entities/monitor';
import { MonitorSchedulingControl } from './MonitorSchedulingControl';
function client(): MonitorClient {
  return {
    getScheduling: vi
      .fn()
      .mockResolvedValue({ kind: 'success', data: { state: 'active' } }),
    setScheduling: vi
      .fn()
      .mockResolvedValue({ kind: 'confirmed', data: { state: 'paused' } }),
    getMonitor: vi.fn(),
    getAvailability: vi.fn(),
    getLatestResult: vi.fn(),
    createMonitor: vi.fn(),
    listMonitors: vi.fn(),
  };
}
it('shows confirmed snapshots and sends explicit Pause and Resume', async () => {
  const api = client();
  render(<MonitorSchedulingControl id="a" client={api} />);
  expect(screen.getByRole('button', { name: 'Pause' })).toBeDisabled();
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Pause' })).toBeEnabled(),
  );
  expect(screen.getByTestId('scheduling-state')).toHaveTextContent('Active');
  expect(
    screen.getByText(
      'Pausing stops new claims. Already claimed checks may still complete.',
    ),
  ).toBeInTheDocument();
  await userEvent.click(screen.getByRole('button', { name: 'Pause' }));
  expect(await screen.findByRole('button', { name: 'Resume' })).toBeEnabled();
  expect(screen.getByTestId('scheduling-state')).toHaveTextContent('Paused');
  api.setScheduling = vi
    .fn()
    .mockResolvedValue({ kind: 'confirmed', data: { state: 'active' } });
  await userEvent.click(screen.getByRole('button', { name: 'Resume' }));
  expect(api.setScheduling).toHaveBeenCalledWith(
    'a',
    'active',
    expect.any(AbortSignal),
  );
});
it('disables commands during writes and treats uncertain readback as a snapshot', async () => {
  const api = client();
  let resolve!: (outcome: SchedulingOutcome) => void;
  api.setScheduling = vi.fn().mockReturnValue(
    new Promise((done) => {
      resolve = done;
    }),
  );
  render(<MonitorSchedulingControl id="a" client={api} />);
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Pause' })).toBeEnabled(),
  );
  await userEvent.click(screen.getByRole('button', { name: 'Pause' }));
  expect(
    screen.getByRole('button', { name: 'Refresh scheduling state' }),
  ).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Pause' })).toBeDisabled();
  await act(async () =>
    resolve({
      kind: 'uncertain',
      error: { kind: 'transport', message: 'write may have completed' },
    }),
  );
  expect(screen.getByRole('alert')).toHaveTextContent('may have completed');
  expect(screen.getByTestId('scheduling-state')).toHaveTextContent('Unknown');
  expect(screen.getByRole('button', { name: 'Pause' })).toBeDisabled();
  await userEvent.click(
    screen.getByRole('button', { name: 'Refresh scheduling state' }),
  );
  await waitFor(() =>
    expect(screen.getByRole('button', { name: 'Pause' })).toBeEnabled(),
  );
  expect(
    screen.getByText(/A read is only a current snapshot/),
  ).toBeInTheDocument();
  expect(api.setScheduling).toHaveBeenCalledOnce();
});
