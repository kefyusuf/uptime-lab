import { describe, expect, it, vi } from 'vitest';
import { createMonitorClient } from './api';
const monitor = {
  id: 'aa29443e-c597-4e7b-9202-a4762e0e04c0',
  targetUrl: 'http://example.com/',
  createdAt: '2026-10-02T12:00:00Z',
};
const signal = () => new AbortController().signal;
describe('Monitor client', () => {
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
