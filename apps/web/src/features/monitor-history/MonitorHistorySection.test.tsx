import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import type { MonitorClient } from '../../entities/monitor';
import { MonitorHistorySection } from './MonitorHistorySection';

function client(): MonitorClient {
  return {
    getHistory: vi.fn().mockResolvedValue({
      kind: 'success',
      data: {
        items: [
          {
            checkId: '7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675',
            resultKind: 'http_response',
            httpStatus: 204,
            durationMs: 123,
            completedAt: '2026-11-01T12:00:00.123456Z',
          },
        ],
        nextCursor: null,
      },
    }),
    getScheduling: vi.fn(),
    setScheduling: vi.fn(),
    deleteMonitor: vi.fn(),
    editTarget: vi.fn(),
    createMonitor: vi.fn(),
    listMonitors: vi.fn(),
    getMonitor: vi.fn(),
    getAvailability: vi.fn(),
    getLatestResult: vi.fn(),
  };
}
it('renders terminal entries and disables traversal without a cursor', async () => {
  const api = client();
  render(<MonitorHistorySection id="a" client={api} />);
  await waitFor(() =>
    expect(screen.getAllByTestId('history-entry')).toHaveLength(1),
  );
  expect(screen.getAllByTestId('history-kind')[0]).toHaveTextContent(
    'http_response',
  );
  expect(
    screen.getByRole('button', { name: 'Next history page' }),
  ).toBeDisabled();
  expect(screen.getByRole('button', { name: 'Refresh history' })).toBeEnabled();
  expect(api.getHistory).toHaveBeenCalledWith(
    'a',
    null,
    expect.any(AbortSignal),
  );
});
it('shows the empty page distinctly and refreshes explicitly', async () => {
  const api = client();
  api.getHistory = vi.fn().mockResolvedValue({
    kind: 'success',
    data: { items: [], nextCursor: null },
  });
  render(<MonitorHistorySection id="a" client={api} />);
  await waitFor(() =>
    expect(screen.getByTestId('history-empty')).toBeInTheDocument(),
  );
  await userEvent.click(
    screen.getByRole('button', { name: 'Refresh history' }),
  );
  expect(api.getHistory).toHaveBeenCalledTimes(2);
});
it('shows read errors with an alert and recovers on refresh', async () => {
  const api = client();
  api.getHistory = vi
    .fn()
    .mockResolvedValueOnce({ kind: 'error', error: { kind: 'transport' } })
    .mockResolvedValue({
      kind: 'success',
      data: { items: [], nextCursor: null },
    });
  render(<MonitorHistorySection id="a" client={api} />);
  expect(await screen.findByRole('alert')).toBeInTheDocument();
  await userEvent.click(
    screen.getByRole('button', { name: 'Refresh history' }),
  );
  await waitFor(() =>
    expect(screen.getByTestId('history-empty')).toBeInTheDocument(),
  );
});
