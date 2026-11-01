import { act, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { MonitorClient } from '../../entities/monitor';
import { useMonitorDelete } from './useMonitorDelete';

const id = 'aa29443e-c597-4e7b-9202-a4762e0e04c0';
const monitor = {
  id,
  targetUrl: 'http://example.com/',
  createdAt: '2026-10-02T12:00:00Z',
};
function fakeClient(
  overrides: Partial<Record<keyof MonitorClient, unknown>> = {},
): MonitorClient {
  return {
    deleteMonitor: vi.fn().mockResolvedValue({ kind: 'deleted' }),
    getMonitor: vi.fn().mockResolvedValue({
      kind: 'error',
      error: { kind: 'http', status: 404, message: 'Monitor not found.' },
    }),
    ...overrides,
  } as unknown as MonitorClient;
}
describe('useMonitorDelete', () => {
  it('gates the delete behind an explicit confirmation', async () => {
    const client = fakeClient();
    const { result } = renderHook(() => useMonitorDelete(id, client));
    expect(result.current.state).toEqual({ kind: 'idle' });
    act(() => result.current.confirmDelete());
    expect(client.deleteMonitor).not.toHaveBeenCalled();
    act(() => result.current.requestDelete());
    expect(result.current.state).toEqual({ kind: 'confirming' });
    act(() => result.current.cancel());
    expect(result.current.state).toEqual({ kind: 'idle' });
    act(() => result.current.confirmDelete());
    expect(client.deleteMonitor).not.toHaveBeenCalled();
    act(() => result.current.requestDelete());
    await act(async () => result.current.confirmDelete());
    expect(client.deleteMonitor).toHaveBeenCalledTimes(1);
    expect(client.deleteMonitor).toHaveBeenCalledWith(
      id,
      expect.any(AbortSignal),
    );
    expect(result.current.state).toEqual({ kind: 'deleted' });
  });
  it('allows one in-flight delete and ignores repeats', async () => {
    const client = fakeClient({
      deleteMonitor: vi.fn(() => new Promise(() => {})),
    });
    const { result } = renderHook(() => useMonitorDelete(id, client));
    act(() => result.current.requestDelete());
    await act(async () => result.current.confirmDelete());
    expect(result.current.state).toEqual({ kind: 'deleting' });
    act(() => result.current.requestDelete());
    act(() => result.current.confirmDelete());
    expect(client.deleteMonitor).toHaveBeenCalledTimes(1);
  });
  it('resolves uncertainty only through explicit refresh', async () => {
    const client = fakeClient({
      deleteMonitor: vi.fn().mockResolvedValue({
        kind: 'uncertain',
        error: { kind: 'transport', message: 'The connection was lost.' },
      }),
    });
    const { result } = renderHook(() => useMonitorDelete(id, client));
    act(() => result.current.requestDelete());
    await act(async () => result.current.confirmDelete());
    expect(result.current.state).toMatchObject({ kind: 'uncertain' });
    expect(client.getMonitor).not.toHaveBeenCalled();
    await act(async () => result.current.refresh());
    expect(client.getMonitor).toHaveBeenCalledTimes(1);
    expect(client.deleteMonitor).toHaveBeenCalledTimes(1);
    expect(result.current.state).toEqual({ kind: 'deleted' });
  });
  it('resolves uncertainty as not deleted when the monitor still exists', async () => {
    const client = fakeClient({
      deleteMonitor: vi.fn().mockResolvedValue({
        kind: 'uncertain',
        error: { kind: 'transport', message: 'The connection was lost.' },
      }),
      getMonitor: vi.fn().mockResolvedValue({ kind: 'success', data: monitor }),
    });
    const { result } = renderHook(() => useMonitorDelete(id, client));
    act(() => result.current.requestDelete());
    await act(async () => result.current.confirmDelete());
    expect(result.current.state).toMatchObject({ kind: 'uncertain' });
    await act(async () => result.current.refresh());
    expect(result.current.state).toEqual({ kind: 'idle' });
    expect(client.deleteMonitor).toHaveBeenCalledTimes(1);
  });
  it('keeps a failed refresh uncertain without deleting again', async () => {
    const client = fakeClient({
      deleteMonitor: vi.fn().mockResolvedValue({
        kind: 'uncertain',
        error: { kind: 'transport', message: 'The connection was lost.' },
      }),
      getMonitor: vi
        .fn()
        .mockResolvedValue({ kind: 'error', error: { kind: 'transport' } }),
    });
    const { result } = renderHook(() => useMonitorDelete(id, client));
    act(() => result.current.requestDelete());
    await act(async () => result.current.confirmDelete());
    await act(async () => result.current.refresh());
    expect(result.current.state).toMatchObject({ kind: 'uncertain' });
    expect(client.deleteMonitor).toHaveBeenCalledTimes(1);
  });
});
