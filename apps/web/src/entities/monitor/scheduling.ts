import { ResponseDecodeError } from './decode';
import type { MonitorScheduling } from './model';
export function decodeMonitorScheduling(value: unknown): MonitorScheduling {
  if (value === null || typeof value !== 'object' || Array.isArray(value))
    throw new ResponseDecodeError();
  const record = value as Record<string, unknown>;
  if (
    Object.keys(record).length !== 1 ||
    !Object.hasOwn(record, 'state') ||
    (record.state !== 'active' && record.state !== 'paused')
  )
    throw new ResponseDecodeError();
  return { state: record.state };
}
