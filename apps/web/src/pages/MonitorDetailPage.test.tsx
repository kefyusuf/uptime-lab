import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import type { MonitorClient } from '../entities/monitor';
import { MonitorDetailPage } from './MonitorDetailPage';
const id = 'aa29443e-c597-4e7b-9202-a4762e0e04c0',
  other = 'bb29443e-c597-4e7b-9202-a4762e0e04c0';
const evaluatedAt = '2026-10-02T12:00:00.123456789Z';
function client(): MonitorClient {
  return {
    createMonitor: vi.fn(),
    listMonitors: vi.fn().mockResolvedValue({
      kind: 'success',
      data: { items: [], nextCursor: null },
    }),
    getMonitor: vi.fn().mockResolvedValue({
      kind: 'success',
      data: {
        id,
        targetUrl: 'http://example.com/<script>unsafe</script>',
        createdAt: evaluatedAt,
      },
    }),
    getAvailability: vi.fn().mockResolvedValue({
      kind: 'success',
      data: {
        status: 'available',
        reason: 'successful_response',
        evaluatedAt,
        evidence: { checkId: id, completedAt: evaluatedAt },
      },
    }),
    getLatestResult: vi.fn().mockResolvedValue({
      kind: 'success',
      data: {
        checkId: other,
        completedAt: evaluatedAt,
        resultKind: 'http_response',
        httpStatus: 200,
        durationMs: 0,
      },
    }),
  };
}
it('shows separately identified snapshots and a plain escaped target; focuses the heading', async () => {
  const api = client();
  const { container } = render(<MonitorDetailPage id={id} client={api} />);
  expect(screen.getByRole('heading', { name: 'Monitor detail' })).toHaveFocus();
  expect(await screen.findByTestId('availability-check-id')).toHaveTextContent(
    id,
  );
  expect(await screen.findByTestId('raw-check-id')).toHaveTextContent(other);
  expect(
    screen.getByText('http://example.com/<script>unsafe</script>'),
  ).toBeInTheDocument();
  expect(container.querySelector('script')).toBeNull();
  expect(container.querySelector('a[href^="http"]')).toBeNull();
  expect(screen.getByText('Assessment at')).toBeInTheDocument();
  expect(screen.getByText('Snapshot; refresh to reassess')).toBeInTheDocument();
  expect(screen.getByText('Completed at')).toBeInTheDocument();
});
it('distinguishes an empty raw result from the server no_result assessment', async () => {
  const api = client();
  api.getLatestResult = vi
    .fn()
    .mockResolvedValue({ kind: 'success', data: null });
  api.getAvailability = vi.fn().mockResolvedValue({
    kind: 'success',
    data: { status: 'unknown', reason: 'no_result', evaluatedAt },
  });
  render(<MonitorDetailPage id={id} client={api} />);
  await waitFor(() =>
    expect(screen.getAllByText('No completed result yet')).toHaveLength(2),
  );
  expect(screen.getByTestId('availability-status')).toHaveTextContent(
    'Unknown',
  );
  expect(screen.queryByTestId('raw-check-id')).toBeNull();
});
it.each([
  [400, 'Invalid Monitor ID.'],
  [404, 'Monitor not found.'],
])(
  'shows a distinct Monitor %i error and omits subordinate cards',
  async (status, message) => {
    const api = client();
    api.getMonitor = vi.fn().mockResolvedValue({
      kind: 'error',
      error: { kind: 'http', status, message: 'untrusted' },
    });
    render(<MonitorDetailPage id={id} client={api} />);
    expect(await screen.findByRole('alert')).toHaveTextContent(message);
    expect(api.getAvailability).not.toHaveBeenCalled();
    expect(
      screen.queryByRole('region', { name: 'Current availability' }),
    ).toBeNull();
  },
);
it('a current Monitor404 removes previously successful snapshots after refresh', async () => {
  const api = client();
  render(<MonitorDetailPage id={id} client={api} />);
  await screen.findByTestId('availability-status');
  api.getMonitor = vi.fn().mockResolvedValue({
    kind: 'error',
    error: { kind: 'http', status: 404, message: 'gone' },
  });
  await userEvent.click(
    screen.getByRole('button', { name: 'Refresh monitor' }),
  );
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'Monitor not found.',
  );
  expect(screen.queryByTestId('availability-status')).toBeNull();
  expect(screen.queryByTestId('raw-check-id')).toBeNull();
});
it('time passing does not reclassify a snapshot or poll', async () => {
  const api = client();
  render(<MonitorDetailPage id={id} client={api} />);
  await screen.findByTestId('availability-status');
  const time = screen.getByLabelText('Assessment time');
  expect(time).toHaveAttribute('datetime', evaluatedAt);
  vi.useFakeTimers();
  try {
    await act(async () => {
      vi.advanceTimersByTime(180000);
    });
    expect(screen.getByTestId('availability-status')).toHaveTextContent(
      'Available',
    );
    expect(time).toHaveAttribute('datetime', evaluatedAt);
    expect(api.getAvailability).toHaveBeenCalledOnce();
  } finally {
    vi.useRealTimers();
  }
});
