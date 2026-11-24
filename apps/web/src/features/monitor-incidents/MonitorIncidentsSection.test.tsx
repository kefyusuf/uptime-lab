import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import type { MonitorClient } from '../../entities/monitor';
import { MonitorIncidentsSection } from './MonitorIncidentsSection';

const openIncident = {
  incidentId: '018f22d3-1d6a-7cc0-a37b-46fc3faff001',
  openedAt: '2026-11-01T12:00:00.123456Z',
  openingReason: 'unexpected_http_status' as const,
  openedByCheckId: '7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675',
};
const resolvedIncident = {
  incidentId: '018f22d3-1d6a-7cc0-a37b-46fc3faff002',
  openedAt: '2026-11-01T11:00:00.123456Z',
  resolvedAt: '2026-11-01T11:05:00.123456Z',
  openingReason: 'probe_failure' as const,
  openedByCheckId: '7d9b2eb0-52bb-4dd7-8934-c5ce30d5c676',
  resolvedByCheckId: '7d9b2eb0-52bb-4dd7-8934-c5ce30d5c677',
};
function client(): MonitorClient {
  return {
    getIncidents: vi.fn().mockResolvedValue({
      kind: 'success',
      data: { items: [openIncident], nextCursor: null },
    }),
    getHistory: vi.fn(),
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
it('sectionRendersOpenBadge', async () => {
  const api = client();
  render(<MonitorIncidentsSection id="a" client={api} />);
  await waitFor(() =>
    expect(screen.getAllByTestId('incident-entry')).toHaveLength(1),
  );
  expect(screen.getByTestId('incident-open-badge')).toHaveTextContent('Open');
  expect(screen.getByTestId('incident-reason')).toHaveTextContent(
    'unexpected_http_status',
  );
  expect(screen.getByLabelText('Opened at')).toHaveAttribute(
    'datetime',
    openIncident.openedAt,
  );
  expect(screen.queryByLabelText('Resolved at')).toBeNull();
  expect(screen.queryByTestId('incident-duration')).toBeNull();
  expect(
    screen.getByRole('button', { name: 'Next incident page' }),
  ).toBeDisabled();
  expect(api.getIncidents).toHaveBeenCalledWith(
    'a',
    null,
    expect.any(AbortSignal),
  );
});
it('sectionRendersResolvedWindow', async () => {
  const api = client();
  api.getIncidents = vi.fn().mockResolvedValue({
    kind: 'success',
    data: { items: [resolvedIncident], nextCursor: null },
  });
  render(<MonitorIncidentsSection id="a" client={api} />);
  await waitFor(() =>
    expect(screen.getAllByTestId('incident-entry')).toHaveLength(1),
  );
  expect(screen.queryByTestId('incident-open-badge')).toBeNull();
  expect(screen.getByLabelText('Resolved at')).toHaveAttribute(
    'datetime',
    resolvedIncident.resolvedAt,
  );
  expect(screen.getByTestId('incident-duration')).toHaveTextContent('300');
  expect(screen.getByTestId('incident-reason')).toHaveTextContent(
    'probe_failure',
  );
});
it('sectionRefreshRecomputes', async () => {
  const api = client();
  api.getIncidents = vi
    .fn()
    .mockResolvedValueOnce({
      kind: 'success',
      data: { items: [openIncident], nextCursor: null },
    })
    .mockResolvedValue({
      kind: 'success',
      data: { items: [resolvedIncident], nextCursor: null },
    });
  render(<MonitorIncidentsSection id="a" client={api} />);
  await waitFor(() =>
    expect(screen.getByTestId('incident-open-badge')).toBeInTheDocument(),
  );
  await userEvent.click(
    screen.getByRole('button', { name: 'Refresh incidents' }),
  );
  await waitFor(() =>
    expect(screen.getByLabelText('Resolved at')).toBeInTheDocument(),
  );
  expect(api.getIncidents).toHaveBeenCalledTimes(2);
});
it('shows the empty page distinctly and recovers from a read error', async () => {
  const api = client();
  api.getIncidents = vi
    .fn()
    .mockResolvedValueOnce({ kind: 'error', error: { kind: 'transport' } })
    .mockResolvedValue({
      kind: 'success',
      data: { items: [], nextCursor: null },
    });
  render(<MonitorIncidentsSection id="a" client={api} />);
  expect(await screen.findByRole('alert')).toBeInTheDocument();
  await userEvent.click(
    screen.getByRole('button', { name: 'Refresh incidents' }),
  );
  await waitFor(() =>
    expect(screen.getByTestId('incidents-empty')).toBeInTheDocument(),
  );
});
