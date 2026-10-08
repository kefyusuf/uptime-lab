import { describe, expect, it, vi } from 'vitest';
import { createMonitorClient } from './api';
const monitor = {
  id: 'aa29443e-c597-4e7b-9202-a4762e0e04c0',
  targetUrl: 'http://example.com/',
  createdAt: '2026-10-02T12:00:00Z',
};
const signal = () => new AbortController().signal;
describe('Monitor client', () => {
  it('bounds a lost scheduling write at12s without retrying', async () => {
    vi.useFakeTimers();
    try {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockImplementation(() => new Promise(() => {}));
      const outcome = createMonitorClient(fetcher).setScheduling(
        monitor.id,
        'paused',
        signal(),
      );
      await vi.advanceTimersByTimeAsync(12000);
      expect(await outcome).toMatchObject({ kind: 'uncertain' });
      expect(fetcher.mock.calls[0][1]?.signal?.aborted).toBe(true);
      expect(fetcher).toHaveBeenCalledOnce();
    } finally {
      vi.useRealTimers();
    }
  });
  it('reads scheduling and confirms only an explicit requested state', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(Response.json({ state: 'active' }))
      .mockResolvedValueOnce(Response.json({ state: 'paused' }));
    const client = createMonitorClient(fetcher);
    expect(await client.getScheduling(monitor.id, signal())).toEqual({
      kind: 'success',
      data: { state: 'active' },
    });
    expect(await client.setScheduling(monitor.id, 'paused', signal())).toEqual({
      kind: 'confirmed',
      data: { state: 'paused' },
    });
    expect(fetcher).toHaveBeenLastCalledWith(
      '/api/monitors/' + monitor.id + '/scheduling',
      expect.objectContaining({
        method: 'PUT',
        body: '{"state":"paused"}',
        headers: { 'Content-Type': 'application/json' },
        cache: 'no-store',
        credentials: 'omit',
        redirect: 'error',
      }),
    );
    expect(fetcher).toHaveBeenCalledTimes(2);
  });
  it.each([200, 201, 202, 204, 500, 502, 504])(
    'does not confirm scheduling status %d with a mismatched or empty body',
    async (status) => {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockResolvedValue(
          status === 204
            ? new Response(null, { status })
            : Response.json({ state: 'active' }, { status }),
        );
      expect(
        await createMonitorClient(fetcher).setScheduling(
          monitor.id,
          'paused',
          signal(),
        ),
      ).toMatchObject({ kind: 'uncertain' });
      expect(fetcher).toHaveBeenCalledOnce();
    },
  );
  it.each([400, 404, 413, 415])(
    'requires reread after a known scheduling rejection %d',
    async (status) => {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockResolvedValue(
          Response.json({ title: 'secret details', status }, { status }),
        );
      const outcome = await createMonitorClient(fetcher).setScheduling(
        monitor.id,
        'paused',
        signal(),
      );
      expect(outcome).toMatchObject({ kind: 'rejected', error: { status } });
      expect(JSON.stringify(outcome)).not.toContain('secret');
      expect(fetcher).toHaveBeenCalledOnce();
    },
  );
  it.each([
    () => Promise.reject(Error('secret')),
    () => Promise.resolve(Response.json({ state: 'paused', extra: 1 })),
    () => Promise.resolve(new Response('broken', { status: 200 })),
  ])('never retries an uncertain scheduling write', async (response) => {
    const fetcher = vi.fn<typeof fetch>().mockImplementation(response);
    expect(
      await createMonitorClient(fetcher).setScheduling(
        monitor.id,
        'paused',
        signal(),
      ),
    ).toMatchObject({ kind: 'uncertain' });
    expect(fetcher).toHaveBeenCalledOnce();
  });
  it.each([
    'http://[fe80::1%25eth0]/',
    'http://example.com:65536/',
    'http://127.0.0.999/',
  ])('reads a page containing Go-accepted target %s', async (targetUrl) => {
    const item = { ...monitor, targetUrl };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(
        Response.json({ items: [item], nextCursor: 'A'.repeat(88) }),
      );
    expect(
      await createMonitorClient(fetcher).listMonitors(
        { limit: 20, cursor: null },
        signal(),
      ),
    ).toEqual({
      kind: 'success',
      data: { items: [item], nextCursor: 'A'.repeat(88) },
    });
  });
  it('bounds inventory requests with the existing12s cancellation', async () => {
    vi.useFakeTimers();
    try {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockImplementation(() => new Promise(() => {}));
      const result = createMonitorClient(fetcher).listMonitors(
        { limit: 20, cursor: null },
        signal(),
      );
      await vi.advanceTimersByTimeAsync(12000);
      expect(await result).toMatchObject({
        kind: 'error',
        error: { kind: 'transport' },
      });
      expect(fetcher.mock.calls[0][1]?.signal?.aborted).toBe(true);
      expect(fetcher).toHaveBeenCalledTimes(1);
    } finally {
      vi.useRealTimers();
    }
  });
  it('reads inventory through a canonical bounded query without credentials', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json({ items: [monitor], nextCursor: null }));
    expect(
      await createMonitorClient(fetcher).listMonitors(
        { limit: 20, cursor: null },
        signal(),
      ),
    ).toEqual({
      kind: 'success',
      data: { items: [monitor], nextCursor: null },
    });
    expect(fetcher).toHaveBeenCalledWith(
      '/api/monitors?limit=20',
      expect.objectContaining({
        method: 'GET',
        cache: 'no-store',
        credentials: 'omit',
        redirect: 'error',
      }),
    );
    const cursor = 'A'.repeat(88);
    await createMonitorClient(fetcher).listMonitors(
      { limit: 20, cursor },
      signal(),
    );
    expect(fetcher).toHaveBeenLastCalledWith(
      '/api/monitors?limit=20&cursor=' + cursor,
      expect.anything(),
    );
  });
  it('rejects invalid inventory caller input before fetch', async () => {
    const fetcher = vi.fn<typeof fetch>();
    for (const input of [
      { limit: 0, cursor: null },
      { limit: 51, cursor: null },
      { limit: 1.5, cursor: null },
      { limit: 20, cursor: 'invalid' },
    ])
      expect(
        (await createMonitorClient(fetcher).listMonitors(input, signal())).kind,
      ).toBe('error');
    expect(fetcher).not.toHaveBeenCalled();
  });
  it('rejects inventory204 and arbitrary error details', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(
        Response.json({ detail: 'secret' }, { status: 500 }),
      );
    const api = createMonitorClient(fetcher);
    expect(
      (await api.listMonitors({ limit: 20, cursor: null }, signal())).kind,
    ).toBe('error');
    expect(
      JSON.stringify(
        await api.listMonitors({ limit: 20, cursor: null }, signal()),
      ),
    ).not.toContain('secret');
  });
  it('shows fixed copy for the approved oversized500', async () => {
    const fetcher = vi.fn<typeof fetch>().mockResolvedValue(
      Response.json(
        {
          type: 'about:blank',
          title: 'Internal Server Error',
          status: 500,
          detail: 'A registered monitor exceeds the inventory response limit.',
        },
        { status: 500 },
      ),
    );
    expect(
      await createMonitorClient(fetcher).listMonitors(
        { limit: 20, cursor: null },
        signal(),
      ),
    ).toMatchObject({
      kind: 'error',
      error: {
        status: 500,
        message: 'A registered monitor exceeds the inventory response limit.',
      },
    });
  });
  it('rejects200 JSON null as malformed raw result', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json(null));
    expect(
      (await createMonitorClient(fetcher).getLatestResult(monitor.id, signal()))
        .kind,
    ).toBe('error');
  });
  it('creates from a validated201 response exactly once', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json(monitor, { status: 201 }));
    expect(
      await createMonitorClient(fetcher).createMonitor(
        monitor.targetUrl,
        signal(),
      ),
    ).toEqual({ kind: 'created', monitor });
    expect(fetcher).toHaveBeenCalledTimes(1);
    expect(fetcher).toHaveBeenCalledWith(
      '/api/monitors',
      expect.objectContaining({
        method: 'POST',
        body: JSON.stringify({ targetUrl: monitor.targetUrl }),
      }),
    );
  });
  it.each([400, 403, 405, 413, 415, 422])(
    'rejects definitive input/access response %i',
    async (status) => {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockResolvedValue(
          Response.json({ detail: 'secret database message' }, { status }),
        );
      const result = await createMonitorClient(fetcher).createMonitor(
        monitor.targetUrl,
        signal(),
      );
      expect(result.kind).toBe('rejected');
      expect(JSON.stringify(result)).not.toContain('secret');
    },
  );
  it.each([500, 502, 504, 429, 200])(
    'treats unconfirmed response %i as uncertain without retry',
    async (status) => {
      const fetcher = vi
        .fn<typeof fetch>()
        .mockResolvedValue(Response.json({ detail: 'secret' }, { status }));
      const result = await createMonitorClient(fetcher).createMonitor(
        monitor.targetUrl,
        signal(),
      );
      expect(result).toEqual({
        kind: 'uncertain',
        message: expect.stringContaining('may have been created'),
      });
      expect(fetcher).toHaveBeenCalledTimes(1);
    },
  );
  it('treats invalid201 and disconnected creation as uncertain', async () => {
    for (const fetcher of [
      vi
        .fn<typeof fetch>()
        .mockResolvedValue(Response.json({}, { status: 201 })),
      vi.fn<typeof fetch>().mockRejectedValue(new Error('internal hostname')),
    ]) {
      const result = await createMonitorClient(fetcher).createMonitor(
        monitor.targetUrl,
        signal(),
      );
      expect(result.kind).toBe('uncertain');
      expect(JSON.stringify(result)).not.toContain('internal hostname');
    }
  });
  it('represents204 as empty only for latest result', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockImplementation(async () => new Response(null, { status: 204 }));
    const client = createMonitorClient(fetcher);
    expect(await client.getLatestResult(monitor.id, signal())).toEqual({
      kind: 'success',
      data: null,
    });
    expect((await client.getMonitor(monitor.id, signal())).kind).toBe('error');
    expect((await client.getAvailability(monitor.id, signal())).kind).toBe(
      'error',
    );
  });
  it('retains read error status and hides server details', async () => {
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json({ detail: 'secret' }, { status: 404 }));
    expect(
      await createMonitorClient(fetcher).getMonitor(monitor.id, signal()),
    ).toEqual({
      kind: 'error',
      error: { kind: 'http', status: 404, message: expect.any(String) },
    });
  });
  it('reads independent availability and raw result without joining IDs', async () => {
    const assessment = {
      status: 'unknown',
      reason: 'no_result',
      evaluatedAt: '2026-10-02T12:00:00Z',
    };
    const fetcher = vi
      .fn<typeof fetch>()
      .mockResolvedValue(Response.json(assessment));
    expect(
      await createMonitorClient(fetcher).getAvailability(monitor.id, signal()),
    ).toEqual({ kind: 'success', data: assessment });
    expect(fetcher).toHaveBeenCalledWith(
      '/api/monitors/' + monitor.id + '/availability',
      expect.any(Object),
    );
  });
});
