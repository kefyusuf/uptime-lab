import { decodeMonitor, ResponseDecodeError } from './decode';
import type { MonitorInventoryPage } from './model';

export function decodeInventoryPage(
  value: unknown,
  limit: number,
): MonitorInventoryPage {
  const fail = (): never => {
    throw new ResponseDecodeError();
  };
  if (
    !Number.isInteger(limit) ||
    limit < 1 ||
    limit > 50 ||
    value === null ||
    typeof value !== 'object' ||
    Array.isArray(value)
  )
    return fail();
  const page = value as Record<string, unknown>;
  if (
    Object.keys(page).length !== 2 ||
    !Object.hasOwn(page, 'items') ||
    !Object.hasOwn(page, 'nextCursor') ||
    !Array.isArray(page.items) ||
    page.items.length > limit
  )
    return fail();
  const nextCursor = page.nextCursor;
  if (
    nextCursor !== null &&
    (typeof nextCursor !== 'string' ||
      nextCursor.length !== 88 ||
      !/^[A-Za-z0-9_-]{88}$/.test(nextCursor))
  )
    return fail();
  if (page.items.length === 0 && nextCursor !== null) return fail();
  const ids = new Set<string>();
  const items = page.items.map((value) => {
    const monitor = decodeMonitor(value);
    if (
      !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/.test(
        monitor.id,
      ) ||
      monitor.id === '00000000-0000-0000-0000-000000000000' ||
      ids.has(monitor.id) ||
      !/^\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d(?:\.\d{1,6})?Z$/.test(
        monitor.createdAt,
      ) ||
      monitor.createdAt.startsWith('0000-') ||
      /^0001-01-01T00:00:00(?:\.0+)?Z$/.test(monitor.createdAt)
    )
      return fail();
    // Target text is owned by Go; WHATWG URL rules differ from its parser.
    ids.add(monitor.id);
    return monitor;
  });
  return { items, nextCursor };
}

export function decodeInventoryProblem(value: unknown): 'oversized' | null {
  if (value === null || typeof value !== 'object' || Array.isArray(value))
    return null;
  const problem = value as Record<string, unknown>;
  return Object.keys(problem).length === 4 &&
    problem.type === 'about:blank' &&
    problem.title === 'Internal Server Error' &&
    problem.status === 500 &&
    problem.detail ===
      'A registered monitor exceeds the inventory response limit.'
    ? 'oversized'
    : null;
}
