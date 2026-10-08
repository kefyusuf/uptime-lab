import { expect, it } from 'vitest';
import { decodeMonitorScheduling } from './scheduling';
it.each(['active', 'paused'])('decodes scheduling state %s', (state) => {
  expect(decodeMonitorScheduling({ state })).toEqual({ state });
});
it.each([
  null,
  [],
  {},
  'paused',
  { state: null },
  { state: 1 },
  { state: 'Paused' },
  { state: 'disabled' },
  { State: 'paused' },
  { state: 'paused', extra: true },
])('rejects noncontracted scheduling %o', (value) => {
  expect(() => decodeMonitorScheduling(value)).toThrow('invalid response');
});
