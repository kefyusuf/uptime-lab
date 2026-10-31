import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  decodeAvailability,
  decodeLatestResult,
  decodeMonitor,
  ResponseDecodeError,
} from './decode';
const directory = resolve('../../contracts/fixtures/public');
const fixture = (name: string): unknown =>
  JSON.parse(readFileSync(resolve(directory, name), 'utf8'));
const monitor = {
  id: 'aa29443e-c597-4e7b-9202-a4762e0e04c0',
  targetUrl: 'https://example.com/health?region=eu',
  createdAt: '2026-10-02T12:00:00Z',
};
describe('public response decoding', () => {
  it('accepts every committed availability and raw-result fixture unchanged', () => {
    for (const name of readdirSync(directory)) {
      const value = fixture(name);
      if (name.startsWith('availability-'))
        expect(decodeAvailability(value)).toEqual(value);
      if (name.startsWith('latest-result-'))
        expect(decodeLatestResult(value)).toEqual(value);
    }
  });
  it('preserves server target text without imposing browser URL policy', () => {
    const value = {
      ...monitor,
      targetUrl: 'http://example.com/path with space',
    };
    expect(decodeMonitor(value)).toEqual(value);
  });
  it.each([
    { ...monitor, id: 'invalid' },
    { ...monitor, createdAt: '2026-02-30T12:00:00Z' },
    { ...monitor, extra: true },
    { ...monitor, targetUrl: 1 },
    null,
  ])('rejects invalid Monitor %j', (value) => {
    expect(() => decodeMonitor(value)).toThrow(ResponseDecodeError);
  });
  it('requires no-result to omit evidence', () => {
    const value = fixture('availability-unknown-no-result.json') as Record<
      string,
      unknown
    >;
    expect(decodeAvailability(value)).not.toHaveProperty('evidence');
    expect(() => decodeAvailability({ ...value, evidence: {} })).toThrow(
      ResponseDecodeError,
    );
  });
  it('rejects incompatible reason and missing evidence', () => {
    const value = fixture('availability-available.json') as Record<
      string,
      unknown
    >;
    for (const mutation of [
      { reason: 'future_reason' },
      { status: 'unknown' },
      { evidence: undefined },
    ])
      expect(() => decodeAvailability({ ...value, ...mutation })).toThrow(
        ResponseDecodeError,
      );
  });
  it('preserves zero duration and absent timeout fields', () => {
    expect(
      decodeLatestResult(fixture('latest-result-failure.json')),
    ).toHaveProperty('durationMs', 0);
    const value = fixture('latest-result-worker-timeout.json') as object;
    expect(decodeLatestResult(value)).not.toHaveProperty('durationMs');
    expect(() => decodeLatestResult({ ...value, durationMs: 0 })).toThrow(
      ResponseDecodeError,
    );
  });
  it('does not recalculate HTTP or freshness policy', () => {
    const value = fixture('availability-available.json') as Record<
      string,
      unknown
    >;
    expect(
      decodeAvailability({ ...value, evaluatedAt: '2099-10-02T12:00:00Z' }),
    ).toHaveProperty('status', 'available');
    const raw = fixture('latest-result-http-response.json') as object;
    expect(decodeLatestResult({ ...raw, httpStatus: 503 })).toHaveProperty(
      'resultKind',
      'http_response',
    );
  });
  it.each([100, 599])('accepts valid HTTP edge %i', (httpStatus) =>
    expect(
      decodeLatestResult({
        ...(fixture('latest-result-http-response.json') as object),
        httpStatus,
      }),
    ).toHaveProperty('httpStatus', httpStatus),
  );
  it.each([99, 600, 200.5])('rejects invalid HTTP status %j', (httpStatus) =>
    expect(() =>
      decodeLatestResult({
        ...(fixture('latest-result-http-response.json') as object),
        httpStatus,
      }),
    ).toThrow(ResponseDecodeError),
  );
});
