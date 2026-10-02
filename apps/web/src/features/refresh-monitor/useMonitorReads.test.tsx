import { act, renderHook, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import type {
  Availability,
  Monitor,
  MonitorClient,
  ReadResult,
} from '../../entities/monitor';
import { useMonitorReads } from './useMonitorReads';
const a = 'aa29443e-c597-4e7b-9202-a4762e0e04c0',
  b = 'bb29443e-c597-4e7b-9202-a4762e0e04c0';
const monitor: Monitor = {
  id: a,
  targetUrl: 'http://example.com/',
  createdAt: '2026-10-02T12:00:00Z',
};
const available: Availability = {
  status: 'available',
  reason: 'successful_response',
  evaluatedAt: '2026-10-02T12:00:00Z',
  evidence: { checkId: a, completedAt: '2026-10-02T11:59:59Z' },
};
function deferred<T>() {
  let resolve: (value: T) => void = () => {};
  const promise = new Promise<T>((r) => {
    resolve = r;
  });
  return { promise, resolve };
}
function client(): MonitorClient {
  return {
    createMonitor: vi.fn(),
    getMonitor: vi.fn().mockResolvedValue({ kind: 'success', data: monitor }),
    getAvailability: vi
      .fn()
      .mockResolvedValue({ kind: 'success', data: available }),
    getLatestResult: vi.fn().mockResolvedValue({ kind: 'success', data: null }),
  };
}
it('loads immutable Monitor before independent subordinate reads', async () => {
  const pending = deferred<ReadResult<Monitor>>(),
    api = client();
  api.getMonitor = vi.fn().mockReturnValue(pending.promise);
  const { result } = renderHook(() => useMonitorReads(a, api));
  expect(result.current.monitor.kind).toBe('loading');
  expect(api.getAvailability).not.toHaveBeenCalled();
  await act(async () => pending.resolve({ kind: 'success', data: monitor }));
  await waitFor(() => expect(result.current.availability.kind).toBe('success'));
  expect(result.current.latest).toEqual({ kind: 'success', data: null });
});
it('allows one read to fail while the other independently succeeds', async () => {
  const api = client();
  api.getLatestResult = vi.fn().mockResolvedValue({
    kind: 'error',
    error: { kind: 'http', status: 500, message: 'Read failed.' },
  });
  const { result } = renderHook(() => useMonitorReads(a, api));
  await waitFor(() => expect(result.current.availability.kind).toBe('success'));
  expect(result.current.latest.kind).toBe('error');
});
it('refresh clears old verdict and refetches all reads', async () => {
  const api = client();
  const { result } = renderHook(() => useMonitorReads(a, api));
  await waitFor(() => expect(result.current.availability.kind).toBe('success'));
  const pending = deferred<ReadResult<Monitor>>();
  api.getMonitor = vi.fn().mockReturnValue(pending.promise);
  act(() => result.current.refresh());
  expect(result.current.availability.kind).not.toBe('success');
  expect(result.current.monitor.kind).toBe('loading');
  await act(async () => pending.resolve({ kind: 'success', data: monitor }));
  await waitFor(() => expect(result.current.availability.kind).toBe('success'));
  expect(api.getAvailability).toHaveBeenCalledTimes(2);
  expect(api.getLatestResult).toHaveBeenCalledTimes(2);
});
it('suppresses late old available response after a current failed read', async () => {
  const old = deferred<ReadResult<Availability>>(),
    api = client();
  api.getAvailability = vi
    .fn()
    .mockReturnValueOnce(old.promise)
    .mockResolvedValue({
      kind: 'error',
      error: { kind: 'transport', message: 'Read failed.' },
    });
  const { result } = renderHook(() => useMonitorReads(a, api));
  await waitFor(() => expect(api.getAvailability).toHaveBeenCalledOnce());
  act(() => result.current.refresh());
  await waitFor(() => expect(result.current.availability.kind).toBe('error'));
  await act(async () => old.resolve({ kind: 'success', data: available }));
  expect(result.current.availability.kind).toBe('error');
});
it('navigation hides old Monitor immediately and ignores old completions', async () => {
  const old = deferred<ReadResult<Availability>>(),
    api = client();
  api.getAvailability = vi
    .fn()
    .mockReturnValueOnce(old.promise)
    .mockResolvedValue({
      kind: 'success',
      data: { ...available, evidence: { ...available.evidence, checkId: b } },
    });
  api.getMonitor = vi.fn().mockImplementation(async (id: string) => ({
    kind: 'success',
    data: { ...monitor, id },
  }));
  const { result, rerender } = renderHook(
    ({ id }) => useMonitorReads(id, api),
    { initialProps: { id: a } },
  );
  await waitFor(() => expect(api.getAvailability).toHaveBeenCalledOnce());
  rerender({ id: b });
  await waitFor(() =>
    expect(result.current.monitor).toEqual({
      kind: 'success',
      data: { ...monitor, id: b },
    }),
  );
  await act(async () => old.resolve({ kind: 'success', data: available }));
  expect(result.current.availability).toEqual({
    kind: 'success',
    data: { ...available, evidence: { ...available.evidence, checkId: b } },
  });
});
it('Monitor404 prevents subordinate reads and clears prior snapshots', async () => {
  const api = client();
  api.getMonitor = vi.fn().mockResolvedValue({
    kind: 'error',
    error: { kind: 'http', status: 404, message: 'Monitor not found.' },
  });
  const { result } = renderHook(() => useMonitorReads(a, api));
  await waitFor(() => expect(result.current.monitor.kind).toBe('error'));
  expect(api.getAvailability).not.toHaveBeenCalled();
  expect(result.current.availability.kind).toBe('idle');
});
it('unmount aborts in-flight requests', async () => {
  const api = client(),
    pending = deferred<ReadResult<Monitor>>();
  api.getMonitor = vi.fn().mockReturnValue(pending.promise);
  const view = renderHook(() => useMonitorReads(a, api));
  view.unmount();
  const signal = vi.mocked(api.getMonitor).mock.calls[0][1];
  expect(signal.aborted).toBe(true);
});
