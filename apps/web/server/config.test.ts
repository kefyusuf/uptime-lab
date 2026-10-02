import { expect, it } from 'vitest';
import { loadWebConfig } from './config.js';
it('sets exact default origins and limits', () => {
  expect(loadWebConfig({})).toEqual({
    browserPort: 4173,
    allowedOrigins: ['http://127.0.0.1:4173', 'http://localhost:4173'],
    upstreamOrigin: 'http://api:8080',
    maxHeaderBytes: 16384,
    maxRequestBytes: 65536,
    maxResponseBytes: 262144,
    headerTimeoutMs: 5000,
    requestTimeoutMs: 10000,
    upstreamTimeoutMs: 10000,
  });
});
it.each(['0', '1023', '65536', '1.5', '04173', '4173x', '', '*'])(
  'rejects invalid port %j',
  (port) =>
    expect(() => loadWebConfig({ UPTIME_LAB_WEB_PORT: port })).toThrow(),
);
it('aligns a custom valid port', () =>
  expect(loadWebConfig({ UPTIME_LAB_WEB_PORT: '4300' }).allowedOrigins).toEqual(
    ['http://127.0.0.1:4300', 'http://localhost:4300'],
  ));
