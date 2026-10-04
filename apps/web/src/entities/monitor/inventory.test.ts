import { expect, it } from 'vitest';
import { decodeInventoryPage, decodeInventoryProblem } from './inventory';
import { ResponseDecodeError } from './decode';
const monitor = {
  id: '018f22d3-1d6a-7cc0-a37b-46fc3fafdcb2',
  targetUrl: 'http://web/?a=é&x=<script>',
  createdAt: '2026-10-05T00:00:00.123456Z',
};
const token =
  'MXwyMDI2LTEwLTA1VDAwOjAwOjAwLjEyMzQ1Nlp8MDE4ZjIyZDMtMWQ2YS03Y2MwLWEzN2ItNDZmYzNmYWZkY2Iy';
it('decodes complete targets and keeps the opaque cursor', () => {
  expect(
    decodeInventoryPage({ items: [monitor], nextCursor: token }, 20),
  ).toEqual({ items: [monitor], nextCursor: token });
  expect(decodeInventoryPage({ items: [], nextCursor: null }, 20)).toEqual({
    items: [],
    nextCursor: null,
  });
});
it.each([
  null,
  {},
  { items: [] },
  { items: [], nextCursor: null, extra: true },
  { items: [], nextCursor: token },
  { items: [monitor, monitor], nextCursor: null },
  { items: [monitor], nextCursor: token + '=' },
  { items: [monitor], nextCursor: 'bad' },
  { items: [{ ...monitor, extra: 1 }], nextCursor: null },
  {
    items: [{ ...monitor, id: '00000000-0000-0000-0000-000000000000' }],
    nextCursor: null,
  },
  { items: [{ ...monitor, id: monitor.id.toUpperCase() }], nextCursor: null },
  {
    items: [{ ...monitor, createdAt: '2026-02-30T00:00:00Z' }],
    nextCursor: null,
  },
  {
    items: [{ ...monitor, createdAt: '2026-10-05T00:00:00.1234567Z' }],
    nextCursor: null,
  },
  { items: [{ ...monitor, targetUrl: 'file:///private' }], nextCursor: null },
])('rejects malformed inventory envelope %#', (value) => {
  expect(() => decodeInventoryPage(value, 20)).toThrow(ResponseDecodeError);
});
it('bounds row count to the requested page', () => {
  expect(() =>
    decodeInventoryPage(
      {
        items: [
          monitor,
          { ...monitor, id: '018f22d3-1d6a-7cc0-a37b-46fc3fafdcb3' },
        ],
        nextCursor: null,
      },
      1,
    ),
  ).toThrow(ResponseDecodeError);
});
it('recognizes only the closed approved oversized Problem', () => {
  const problem = {
    type: 'about:blank',
    title: 'Internal Server Error',
    status: 500,
    detail: 'A registered monitor exceeds the inventory response limit.',
  };
  expect(decodeInventoryProblem(problem)).toBe('oversized');
  for (const value of [
    null,
    { ...problem, detail: 'secret' },
    { ...problem, extra: 'secret' },
    { ...problem, status: 400 },
    { ...problem, type: 'private' },
    { ...problem, title: 'secret' },
  ])
    expect(decodeInventoryProblem(value)).toBeNull();
});
