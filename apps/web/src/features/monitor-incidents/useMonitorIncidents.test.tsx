import { act, renderHook, waitFor } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { MonitorClient } from '../../entities/monitor';
import { useMonitorIncidents } from './useMonitorIncidents';

const id = 'aa29443e-c597-4e7b-9202-a4762e0e04c0';
const entry = {
  incidentId: '018f22d3-1d6a-7cc0-a37b-46fc3faff001',
  openedAt: '2026-11-01T12:00:00.123456Z',
  openingReason: 'unexpected_http_status' as const,
  openedByCheckId: '7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675',
};
const cursor = 'C'.repeat(88);
function fakeClient(overrides: Partial<MonitorClient> = {}): MonitorClient {
  return {
    getIncidents: vi.fn().mockResolvedValue({
      kind: 'success',
      data: { items: [entry], nextCursor: cursor },
    }),
    ...overrides,
  } as unknown as MonitorClient;
}
describe('useMonitorIncidents', () => {
  it('reads once and traverses with exact cursor bytes', async () => {
    const client = fakeClient();
    const { result } = renderHook(() => useMonitorIncidents(id, client));
    await waitFor(() => expect(result.current.state.kind).toBe('ready'));
    expect(client.getIncidents).toHaveBeenCalledTimes(1);
    expect(client.getIncidents).toHaveBeenCalledWith(
      id,
      null,
      expect.any(AbortSignal),
    );
    const before = vi.mocked(client.getIncidents).mock.calls[0][2];
    act(() => result.current.next(cursor));
    await waitFor(() => expect(result.current.state.kind).toBe('ready'));
    expect(client.getIncidents).toHaveBeenCalledTimes(2);
    expect(vi.mocked(client.getIncidents).mock.calls[1][1]).toBe(cursor);
    expect(before.aborted).toBe(true);
  });
  it('fences stale responses after navigation', async () => {
    const client = fakeClient({
      getIncidents: vi.fn(() => new Promise<never>(() => {})),
    });
    const { result, rerender } = renderHook(
      ({ monitor }: { monitor: string }) =>
        useMonitorIncidents(monitor, client),
      {
        initialProps: { monitor: id },
      },
    );
    await waitFor(() => expect(result.current.state.kind).toBe('loading'));
    rerender({ monitor: 'bb29443e-c597-4e7b-9202-a4762e0e04c0' });
    expect(result.current.state).toEqual({ kind: 'loading' });
  });
  it('does not retry automatically after a read error', async () => {
    const client = fakeClient({
      getIncidents: vi
        .fn()
        .mockResolvedValue({ kind: 'error', error: { kind: 'transport' } }),
    });
    const { result } = renderHook(() => useMonitorIncidents(id, client));
    await waitFor(() => expect(result.current.state.kind).toBe('read-error'));
    expect(client.getIncidents).toHaveBeenCalledTimes(1);
    act(() => result.current.refresh());
    await waitFor(() => expect(client.getIncidents).toHaveBeenCalledTimes(2));
  });
  it('keeps an empty page distinct from an error', async () => {
    const client = fakeClient({
      getIncidents: vi.fn().mockResolvedValue({
        kind: 'success',
        data: { items: [], nextCursor: null },
      }),
    });
    const { result } = renderHook(() => useMonitorIncidents(id, client));
    await waitFor(() => expect(result.current.state.kind).toBe('ready'));
    if (result.current.state.kind !== 'ready') throw new Error('not ready');
    expect(result.current.state.data.items).toEqual([]);
    expect(result.current.state.data.nextCursor).toBeNull();
  });
});
