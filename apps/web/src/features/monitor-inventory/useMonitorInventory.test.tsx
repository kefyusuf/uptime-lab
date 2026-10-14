import { act, renderHook, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import type {
  MonitorClient,
  MonitorInventoryPage,
  ReadResult,
} from '../../entities/monitor';
import { useMonitorInventory } from './useMonitorInventory';
const cursor = 'A'.repeat(88);
const row = {
  id: '018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2',
  targetUrl: 'http://web/one',
  createdAt: '2026-10-05T00:00:00Z',
};
function client(): MonitorClient {
  return {
    getScheduling: vi.fn(),
    setScheduling: vi.fn(),
    listMonitors: vi.fn(),
    createMonitor: vi.fn(),
    getMonitor: vi.fn(),
    getAvailability: vi.fn(),
    getLatestResult: vi.fn(),
  };
}
function deferred() {
  let resolve!: (value: ReadResult<MonitorInventoryPage>) => void;
  const promise = new Promise<ReadResult<MonitorInventoryPage>>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}
it('Next replaces the page and clears rows while pending without health fan-out', async () => {
  const api = client(),
    pending = deferred();
  api.listMonitors = vi
    .fn()
    .mockResolvedValueOnce({
      kind: 'success',
      data: { items: [row], nextCursor: cursor },
    })
    .mockReturnValueOnce(pending.promise);
  const { result } = renderHook(() => useMonitorInventory(api));
  await waitFor(() => expect(result.current.state.kind).toBe('success'));
  act(() => result.current.next());
  expect(result.current.state.kind).toBe('loading');
  await act(async () =>
    pending.resolve({
      kind: 'success',
      data: {
        items: [{ ...row, targetUrl: 'http://web/two' }],
        nextCursor: null,
      },
    }),
  );
  expect(result.current.state).toEqual({
    kind: 'success',
    data: {
      items: [{ ...row, targetUrl: 'http://web/two' }],
      nextCursor: null,
    },
  });
  expect(api.listMonitors).toHaveBeenLastCalledWith(
    { limit: 20, cursor },
    expect.any(AbortSignal),
  );
  expect(api.getAvailability).not.toHaveBeenCalled();
  expect(api.getLatestResult).not.toHaveBeenCalled();
  expect(api.getScheduling).not.toHaveBeenCalled();
  expect(api.setScheduling).not.toHaveBeenCalled();
  expect(api.createMonitor).not.toHaveBeenCalled();
});
it('Refresh aborts Next and wins over a late response; unmount cancels', async () => {
  const api = client(),
    next = deferred(),
    fresh = deferred();
  const signals: AbortSignal[] = [];
  api.listMonitors = vi
    .fn()
    .mockImplementationOnce((_input, signal) => {
      signals.push(signal);
      return Promise.resolve({
        kind: 'success',
        data: { items: [row], nextCursor: cursor },
      });
    })
    .mockImplementationOnce((_input, signal) => {
      signals.push(signal);
      return next.promise;
    })
    .mockImplementationOnce((_input, signal) => {
      signals.push(signal);
      return fresh.promise;
    });
  const { result, unmount } = renderHook(() => useMonitorInventory(api));
  await waitFor(() => expect(result.current.state.kind).toBe('success'));
  act(() => result.current.next());
  act(() => result.current.refresh());
  expect(signals[1].aborted).toBe(true);
  expect(signals.filter((s) => !s.aborted)).toHaveLength(1);
  await act(async () =>
    next.resolve({ kind: 'success', data: { items: [row], nextCursor: null } }),
  );
  expect(result.current.state.kind).toBe('loading');
  await act(async () =>
    fresh.resolve({ kind: 'success', data: { items: [], nextCursor: null } }),
  );
  expect(result.current.cursor).toBeNull();
  expect(result.current.state).toEqual({
    kind: 'success',
    data: { items: [], nextCursor: null },
  });
  expect(api.listMonitors).toHaveBeenLastCalledWith(
    { limit: 20, cursor: null },
    expect.any(AbortSignal),
  );
  unmount();
  expect(signals.at(-1)?.aborted).toBe(true);
});
it('Retry uses the attempted anchor and errors remain errors', async () => {
  const api = client();
  api.listMonitors = vi
    .fn()
    .mockResolvedValueOnce({
      kind: 'success',
      data: { items: [row], nextCursor: cursor },
    })
    .mockResolvedValue({
      kind: 'error',
      error: { kind: 'http', status: 500, message: 'Read failed.' },
    });
  const { result } = renderHook(() => useMonitorInventory(api));
  await waitFor(() => expect(result.current.state.kind).toBe('success'));
  act(() => result.current.next());
  await waitFor(() => expect(result.current.state.kind).toBe('error'));
  act(() => result.current.retry());
  expect(api.listMonitors).toHaveBeenLastCalledWith(
    { limit: 20, cursor },
    expect.any(AbortSignal),
  );
  await waitFor(() => expect(result.current.state.kind).toBe('error'));
});
