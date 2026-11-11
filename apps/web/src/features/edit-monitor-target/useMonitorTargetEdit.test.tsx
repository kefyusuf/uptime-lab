import { act, renderHook } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import type { MonitorClient } from '../../entities/monitor';
import { useMonitorTargetEdit } from './useMonitorTargetEdit';

const id = 'aa29443e-c597-4e7b-9202-a4762e0e04c0';
const monitor = {
  id,
  targetUrl: 'http://example.com/',
  createdAt: '2026-10-02T12:00:00Z',
};
const edited = { ...monitor, targetUrl: 'https://example.com/edited' };
function fakeClient(
  overrides: Partial<Record<keyof MonitorClient, unknown>> = {},
): MonitorClient {
  return {
    editTarget: vi.fn().mockResolvedValue({ kind: 'edited', data: edited }),
    getMonitor: vi.fn().mockResolvedValue({ kind: 'success', data: monitor }),
    ...overrides,
  } as unknown as MonitorClient;
}
describe('useMonitorTargetEdit', () => {
  it('gates save behind the editing state', async () => {
    const client = fakeClient();
    const { result } = renderHook(() => useMonitorTargetEdit(id, client));
    expect(result.current.state).toEqual({ kind: 'idle' });
    act(() => result.current.save('https://example.com/edited'));
    expect(client.editTarget).not.toHaveBeenCalled();
    expect(result.current.state).toEqual({ kind: 'idle' });
    act(() => result.current.startEdit());
    expect(result.current.state).toEqual({ kind: 'editing', error: null });
    act(() => result.current.cancel());
    expect(result.current.state).toEqual({ kind: 'idle' });
    act(() => result.current.save('https://example.com/edited'));
    expect(client.editTarget).not.toHaveBeenCalled();
    act(() => result.current.startEdit());
    await act(async () => result.current.save('https://example.com/edited'));
    expect(result.current.state).toEqual({ kind: 'saved' });
    act(() => result.current.save('https://example.com/other'));
    expect(client.editTarget).toHaveBeenCalledTimes(1);
    act(() => result.current.startEdit());
    expect(result.current.state).toEqual({ kind: 'editing', error: null });
  });
  it('allows one in-flight edit and ignores repeats', async () => {
    const client = fakeClient({
      editTarget: vi.fn(() => new Promise(() => {})),
    });
    const { result } = renderHook(() => useMonitorTargetEdit(id, client));
    act(() => result.current.startEdit());
    await act(async () => result.current.save('https://example.com/edited'));
    expect(result.current.state).toEqual({ kind: 'saving' });
    act(() => result.current.save('https://example.com/other'));
    act(() => result.current.startEdit());
    expect(result.current.state).toEqual({ kind: 'saving' });
    expect(client.editTarget).toHaveBeenCalledTimes(1);
  });
  it('keeps an uncertain edit unresolved until the explicit refresh', async () => {
    const client = fakeClient({
      editTarget: vi.fn().mockResolvedValueOnce({
        kind: 'uncertain',
        error: {
          kind: 'transport',
          message: 'The target edit outcome is unknown. Refresh to check.',
        },
      }),
      getMonitor: vi.fn().mockResolvedValue({ kind: 'success', data: edited }),
    });
    const { result } = renderHook(() => useMonitorTargetEdit(id, client));
    act(() => result.current.startEdit());
    await act(async () => result.current.save('https://example.com/edited'));
    expect(result.current.state).toMatchObject({ kind: 'uncertain' });
    await act(async () => {
      await Promise.resolve();
      await Promise.resolve();
    });
    expect(result.current.state).toMatchObject({ kind: 'uncertain' });
    act(() => result.current.cancel());
    expect(result.current.state).toMatchObject({ kind: 'uncertain' });
    await act(async () => result.current.refresh());
    expect(result.current.state).toEqual({ kind: 'saved' });
    expect(client.getMonitor).toHaveBeenCalledTimes(1);
  });
  it('resolves an uncertain edit that did not apply back to idle', async () => {
    const client = fakeClient({
      editTarget: vi.fn().mockResolvedValueOnce({
        kind: 'uncertain',
        error: {
          kind: 'transport',
          message: 'The target edit outcome is unknown. Refresh to check.',
        },
      }),
    });
    const { result } = renderHook(() => useMonitorTargetEdit(id, client));
    act(() => result.current.startEdit());
    await act(async () => result.current.save('https://example.com/edited'));
    expect(result.current.state).toMatchObject({ kind: 'uncertain' });
    await act(async () => result.current.refresh());
    expect(result.current.state).toEqual({ kind: 'idle' });
    expect(client.getMonitor).toHaveBeenCalledTimes(1);
  });
  it('keeps a failed refresh resolution uncertain', async () => {
    const client = fakeClient({
      editTarget: vi.fn().mockResolvedValue({
        kind: 'uncertain',
        error: {
          kind: 'transport',
          message: 'The target edit outcome is unknown. Refresh to check.',
        },
      }),
      getMonitor: vi.fn().mockResolvedValue({
        kind: 'error',
        error: { kind: 'http', status: 500, message: 'failed' },
      }),
    });
    const { result } = renderHook(() => useMonitorTargetEdit(id, client));
    act(() => result.current.startEdit());
    await act(async () => result.current.save('https://example.com/edited'));
    await act(async () => result.current.refresh());
    expect(result.current.state).toMatchObject({ kind: 'uncertain' });
    expect(client.getMonitor).toHaveBeenCalledTimes(1);
  });
  it('fences stale sessions after the id or client changes', async () => {
    const first = fakeClient({
      editTarget: vi.fn(() => new Promise(() => {})),
    });
    const { result, rerender } = renderHook(
      ({ client }) => useMonitorTargetEdit(id, client),
      { initialProps: { client: first } },
    );
    act(() => result.current.startEdit());
    await act(async () => result.current.save('https://example.com/edited'));
    expect(result.current.state).toEqual({ kind: 'saving' });
    const second = fakeClient();
    rerender({ client: second });
    expect(result.current.state).toEqual({ kind: 'idle' });
    await act(async () => {
      await Promise.resolve();
    });
    expect(result.current.state).toEqual({ kind: 'idle' });
    expect(second.editTarget).not.toHaveBeenCalled();
  });
});
