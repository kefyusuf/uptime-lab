import type { Availability, Evidence, LatestResult, Monitor } from './model';

export class ResponseDecodeError extends Error {
  constructor() {
    super('The server returned an invalid response.');
    this.name = 'ResponseDecodeError';
  }
}
function fail(): never {
  throw new ResponseDecodeError();
}
function record(
  value: unknown,
  keys: readonly string[],
): Record<string, unknown> {
  if (value === null || typeof value !== 'object' || Array.isArray(value))
    return fail();
  const object = value as Record<string, unknown>;
  if (Object.keys(object).some((key) => !keys.includes(key))) return fail();
  return object;
}
function uuid(value: unknown): string {
  if (
    typeof value !== 'string' ||
    !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
      value,
    )
  )
    return fail();
  return value;
}
function timestamp(value: unknown): string {
  if (
    typeof value !== 'string' ||
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?Z$/.test(value)
  )
    return fail();
  const date = new Date(value);
  if (
    !Number.isFinite(date.getTime()) ||
    date.toISOString().slice(0, 19) !== value.slice(0, 19)
  )
    return fail();
  return value;
}
function integer(value: unknown, min: number, max: number): number {
  if (
    typeof value !== 'number' ||
    !Number.isInteger(value) ||
    value < min ||
    value > max
  )
    return fail();
  return value;
}
function evidence(value: unknown): Evidence {
  const object = record(value, ['checkId', 'completedAt']);
  return {
    checkId: uuid(object.checkId),
    completedAt: timestamp(object.completedAt),
  };
}
export function decodeMonitor(value: unknown): Monitor {
  const object = record(value, ['id', 'targetUrl', 'createdAt']);
  if (typeof object.targetUrl !== 'string') return fail();
  return {
    id: uuid(object.id),
    targetUrl: object.targetUrl,
    createdAt: timestamp(object.createdAt),
  };
}
export function decodeLatestResult(value: unknown): LatestResult {
  const object = record(value, [
    'checkId',
    'completedAt',
    'resultKind',
    'httpStatus',
    'durationMs',
  ]);
  const fact = {
    checkId: uuid(object.checkId),
    completedAt: timestamp(object.completedAt),
  };
  if (object.resultKind === 'worker_timeout') {
    if (
      Object.hasOwn(object, 'httpStatus') ||
      Object.hasOwn(object, 'durationMs')
    )
      return fail();
    return { ...fact, resultKind: 'worker_timeout' };
  }
  const durationMs = integer(object.durationMs, 0, 20000);
  if (object.resultKind === 'http_response')
    return {
      ...fact,
      resultKind: 'http_response',
      durationMs,
      httpStatus: integer(object.httpStatus, 100, 599),
    };
  if (Object.hasOwn(object, 'httpStatus')) return fail();
  switch (object.resultKind) {
    case 'dns_error':
    case 'policy_rejected':
    case 'timeout':
    case 'connect_error':
    case 'tls_error':
    case 'protocol_error':
    case 'internal_error':
      return { ...fact, resultKind: object.resultKind, durationMs };
    default:
      return fail();
  }
}
export function decodeAvailability(value: unknown): Availability {
  const object = record(value, ['status', 'reason', 'evaluatedAt', 'evidence']);
  const evaluatedAt = timestamp(object.evaluatedAt);
  if (object.reason === 'no_result') {
    if (object.status !== 'unknown' || Object.hasOwn(object, 'evidence'))
      return fail();
    return { status: 'unknown', reason: 'no_result', evaluatedAt };
  }
  const proof = evidence(object.evidence);
  if (object.status === 'available' && object.reason === 'successful_response')
    return {
      status: 'available',
      reason: 'successful_response',
      evaluatedAt,
      evidence: proof,
    };
  if (
    object.status === 'unavailable' &&
    (object.reason === 'unexpected_http_status' ||
      object.reason === 'probe_failure')
  )
    return {
      status: 'unavailable',
      reason: object.reason,
      evaluatedAt,
      evidence: proof,
    };
  if (object.status === 'unknown') {
    switch (object.reason) {
      case 'policy_rejected':
      case 'execution_failure':
      case 'stale_result':
      case 'future_result':
        return {
          status: 'unknown',
          reason: object.reason,
          evaluatedAt,
          evidence: proof,
        };
    }
  }
  return fail();
}
