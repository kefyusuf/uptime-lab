import { act, renderHook, waitFor } from '@testing-library/react';
import { expect, it, vi } from 'vitest';
import type {
  MonitorClient,
  MonitorScheduling,
  ReadResult,
  SchedulingOutcome,
} from '../../entities/monitor';
import { useMonitorScheduling } from './useMonitorScheduling';
function deferred<T>() {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
}
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
it('starts one read and never changes state optimistically or overlaps a write', async () => {
  const api = client(),
    pending = deferred<SchedulingOutcome>();
  api.setScheduling = vi.fn().mockReturnValue(pending.promise);
  const { result, rerender } = renderHook(() => useMonitorScheduling('a', api));
  expect(result.current.state.kind).toBe('loading');
  await waitFor(() =>
    expect(result.current.state).toEqual({
      kind: 'ready',
      data: { state: 'active' },
    }),
  );
  rerender();
  expect(api.getScheduling).toHaveBeenCalledOnce();
  act(() => {
    result.current.setState('paused');
    result.current.setState('active');
    result.current.refresh();
  });
  expect(result.current.state.kind).toBe('writing');
  expect(api.setScheduling).toHaveBeenCalledOnce();
  expect(api.getScheduling).toHaveBeenCalledOnce();
  await act(async () =>
    pending.resolve({ kind: 'confirmed', data: { state: 'paused' } }),
  );
  expect(result.current.state).toEqual({
    kind: 'ready',
    data: { state: 'paused' },
  });
});
it.each(['uncertain', 'rejected'] as const)(
  'requires explicit reread after %s without retrying',
  async (kind) => {
    const api = client();
    api.setScheduling = vi.fn().mockResolvedValue({
      kind,
      error: { kind: 'http', status: 500, message: 'write failed' },
    });
    const { result } = renderHook(() => useMonitorScheduling('a', api));
    await waitFor(() => expect(result.current.state.kind).toBe('ready'));
    await act(async () => result.current.setState('paused'));
    expect(result.current.state.kind).toBe(
      kind === 'uncertain' ? 'uncertain' : 'read-error',
    );
    act(() => result.current.setState('paused'));
    expect(api.setScheduling).toHaveBeenCalledOnce();
    await act(async () => result.current.refresh());
    await waitFor(() => expect(result.current.state.kind).toBe('ready'));
    expect(api.getScheduling).toHaveBeenCalledTimes(2);
    expect(api.setScheduling).toHaveBeenCalledOnce();
  },
);
it('fences old read responses after monitor navigation and aborts obsolete work', async () => {
  const api = client(),
    old = deferred<ReadResult<MonitorScheduling>>();
  api.getScheduling = vi
    .fn()
    .mockReturnValueOnce(old.promise)
    .mockResolvedValue({ kind: 'success', data: { state: 'paused' } });
  const { result, rerender } = renderHook(
    ({ id }) => useMonitorScheduling(id, api),
    { initialProps: { id: 'a' } },
  );
  const firstSignal = vi.mocked(api.getScheduling).mock.calls[0][1];
  rerender({ id: 'b' });
  await waitFor(() =>
    expect(result.current.state).toEqual({
      kind: 'ready',
      data: { state: 'paused' },
    }),
  );
  expect(firstSignal.aborted).toBe(true);
  await act(async () =>
    old.resolve({ kind: 'success', data: { state: 'active' } }),
  );
  expect(result.current.state).toEqual({
    kind: 'ready',
    data: { state: 'paused' },
  });
});
it('fences an old write after navigation without retrying it', async () => {
  const api = client(),
    write = deferred<SchedulingOutcome>();
  api.setScheduling = vi.fn().mockReturnValue(write.promise);
  const { result, rerender } = renderHook(
    ({ id }) => useMonitorScheduling(id, api),
    { initialProps: { id: 'a' } },
  );
  await waitFor(() => expect(result.current.state.kind).toBe('ready'));
  act(() => result.current.setState('paused'));
  const signal = vi.mocked(api.setScheduling).mock.calls[0][2];
  rerender({ id: 'b' });
  await waitFor(() =>
    expect(result.current.state).toEqual({
      kind: 'ready',
      data: { state: 'active' },
    }),
  );
  expect(signal.aborted).toBe(true);
  await act(async () =>
    write.resolve({ kind: 'confirmed', data: { state: 'paused' } }),
  );
  expect(result.current.state).toEqual({
    kind: 'ready',
    data: { state: 'active' },
  });
  expect(api.setScheduling).toHaveBeenCalledOnce();
});
it('keeps loading and read errors mutation-free', async () => {
  const api = client(),
    pending = deferred<ReadResult<MonitorScheduling>>();
  api.getScheduling = vi.fn().mockReturnValue(pending.promise);
  const { result } = renderHook(() => useMonitorScheduling('a', api));
  act(() => result.current.setState('paused'));
  expect(api.setScheduling).not.toHaveBeenCalled();
  await act(async () =>
    pending.resolve({
      kind: 'error',
      error: { kind: 'http', status: 500, message: 'failed' },
    }),
  );
  expect(result.current.state.kind).toBe('read-error');
  act(() => result.current.setState('paused'));
  expect(api.setScheduling).not.toHaveBeenCalled();
});
