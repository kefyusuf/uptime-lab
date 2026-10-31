import { describe, expect, it, vi } from 'vitest';
import { requestJson } from './http';
describe('HTTP transport', () => {
  it('does not parse bodyless 204', async () => {
    const response = new Response(null, { status: 204 });
    const json = vi.spyOn(response, 'json');
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(response);
    expect(
      await requestJson(fetcher, '/api/monitors/id/latest-result', {}),
    ).toEqual({ status: 204, value: null });
    expect(json).not.toHaveBeenCalled();
  });
  it('rejects malformed successful JSON', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(new Response('{', { status: 200 }));
    await expect(
      requestJson(fetcher, '/api/monitors/id', {}),
    ).rejects.toThrow();
  });
  it('uses no-store and disables redirects', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json({ id: 'id' }));
    await requestJson(fetcher, '/api/monitors/id', {
      signal: new AbortController().signal,
    });
    expect(fetcher).toHaveBeenCalledWith(
      '/api/monitors/id',
      expect.objectContaining({
        cache: 'no-store',
        redirect: 'error',
        credentials: 'omit',
        signal: expect.any(AbortSignal),
      }),
    );
  });
  it('terminates waiting when an adapter ignores cancellation', async () => {
    vi.useFakeTimers();
    try {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockImplementation(() => new Promise(() => {}));
      const assertion = expect(
        requestJson(fetcher, '/api/monitors/id', {}),
      ).rejects.toThrow();
      await vi.advanceTimersByTimeAsync(12000);
      await assertion;
    } finally {
      vi.useRealTimers();
    }
  });
});
