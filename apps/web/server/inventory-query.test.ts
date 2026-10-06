import { expect, it } from 'vitest';
import { validateInventoryQuery } from './inventory-query.js';
const cursor = 'A'.repeat(88);
it.each([
  '',
  'limit=1',
  'limit=20',
  'limit=50',
  `cursor=${cursor}`,
  `limit=20&cursor=${cursor}`,
  `cursor=${cursor}&limit=20`,
])('accepts only bounded inventory grammar %s', (raw) => {
  expect(validateInventoryQuery(raw)).toBe(true);
});
it.each([
  'limit=0',
  'limit=51',
  'limit=020',
  'limit=-1',
  'limit=+1',
  'limit=%32%30',
  'limit=20+',
  'limit=20 ',
  'limit=20\n',
  'limit=20;cursor=x',
  'limit=20&',
  '&limit=20',
  'limit=20&&cursor=x',
  'limit=20&limit=20',
  'cursor=',
  'cursor=' + cursor + '=',
  'cursor=' + cursor + '&cursor=' + cursor,
  'cursor=' + cursor + '?',
  'unknown=1',
  'x'.repeat(128),
  'x'.repeat(129),
  'limit=é',
])('rejects noncanonical inventory grammar %s', (raw) => {
  expect(validateInventoryQuery(raw)).toBe(false);
});
