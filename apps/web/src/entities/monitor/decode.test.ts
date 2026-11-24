import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';
import {
  decodeAvailability,
  decodeIncident,
  decodeIncidentPage,
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
  it('keeps the monitor decoder closed for edit responses', () => {
    expect(decodeMonitor({ ...monitor })).toEqual(monitor);
    expect(() => decodeMonitor({ ...monitor, extra: 1 })).toThrow(
      ResponseDecodeError,
    );
    expect(() => decodeMonitor({ ...monitor, id: 'not-a-uuid' })).toThrow(
      ResponseDecodeError,
    );
  });
  it('decodeAcceptsRecovering', () => {
    const value = fixture('availability-unavailable-recovering.json');
    expect(decodeAvailability(value)).toEqual(value);
  });
  it('decodeAcceptsAwaitingConfirmation', () => {
    const value = fixture('availability-unknown-awaiting-confirmation.json');
    expect(decodeAvailability(value)).toEqual(value);
  });
  it('decodeRejectsMismatchedNewReasons', () => {
    const recovering = fixture(
      'availability-unavailable-recovering.json',
    ) as Record<string, unknown>;
    const awaiting = fixture(
      'availability-unknown-awaiting-confirmation.json',
    ) as Record<string, unknown>;
    for (const mutation of [
      { ...recovering, status: 'available' },
      { ...recovering, status: 'unknown' },
      { ...recovering, evidence: undefined },
      { ...awaiting, status: 'unavailable' },
      { ...awaiting, status: 'available' },
      { ...awaiting, evidence: undefined },
    ])
      expect(() => decodeAvailability(mutation)).toThrow(ResponseDecodeError);
  });
  const incidentPage = (name: string) =>
    fixture(name) as {
      items: Record<string, unknown>[];
      nextCursor: string | null;
    };
  it('decodeAcceptsOpenIncident', () => {
    const page = incidentPage('incidents-open.json');
    expect(decodeIncidentPage(page)).toEqual(page);
    const open = page.items[0];
    expect(decodeIncident(open)).toEqual(open);
    expect(decodeIncident(open)).not.toHaveProperty('resolvedAt');
    expect(decodeIncident(open)).not.toHaveProperty('resolvedByCheckId');
    expect(decodeIncident(open)).toHaveProperty('openedByCheckId');
  });
  it('decodeAcceptsResolvedIncident', () => {
    const page = incidentPage('incidents-resolved.json');
    expect(decodeIncidentPage(page)).toEqual(page);
    const resolved = page.items[0];
    expect(decodeIncident(resolved)).toEqual(resolved);
    expect(decodeIncident(resolved)).toHaveProperty('resolvedAt');
    expect(decodeIncident(resolved)).toHaveProperty('resolvedByCheckId');
  });
  it('decodeAcceptsNullEvidenceLinks', () => {
    const page = incidentPage('incidents-null-evidence.json');
    expect(decodeIncidentPage(page)).toEqual(page);
    const bare = page.items[0];
    expect(decodeIncident(bare)).toEqual(bare);
    expect(decodeIncident(bare)).not.toHaveProperty('openedByCheckId');
    expect(decodeIncident(bare)).not.toHaveProperty('resolvedByCheckId');
    expect(decodeIncidentPage(fixture('incidents-empty-page.json'))).toEqual({
      items: [],
      nextCursor: null,
    });
  });
  it('decodeRejectsMismatchedIncidentShape', () => {
    const open = incidentPage('incidents-open.json').items[0];
    const resolved = incidentPage('incidents-resolved.json').items[0];
    for (const mutation of [
      { ...open, extra: true },
      { ...open, openingReason: 'future_reason' },
      { ...open, incidentId: 'not-a-uuid' },
      { ...open, openedAt: '2026-02-30T12:00:00Z' },
      { ...open, resolvedAt: null },
      { ...open, openedByCheckId: null },
      { ...open, resolvedByCheckId: '7d9b2eb0-52bb-4dd7-8934-c5ce30d5c675' },
      { ...resolved, resolvedAt: '2026-11-01T10:00:00.123456Z' },
      { openedAt: open.openedAt, openingReason: open.openingReason },
    ])
      expect(() => decodeIncident(mutation)).toThrow(ResponseDecodeError);
  });
  it('keeps the incident page closed and ordered', () => {
    const open = incidentPage('incidents-open.json').items[0];
    for (const mutation of [
      { items: [open, open], nextCursor: null },
      { items: [], nextCursor: 'A'.repeat(88) },
      { items: [open], nextCursor: 'short' },
      { items: Array(51).fill(open), nextCursor: null },
      { items: [open], nextCursor: null, extra: true },
    ])
      expect(() => decodeIncidentPage(mutation)).toThrow(ResponseDecodeError);
  });
});
