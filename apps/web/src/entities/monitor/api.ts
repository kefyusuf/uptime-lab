import { requestJson } from '../../shared/api/http';
import {
  decodeAvailability,
  decodeLatestResult,
  decodeMonitor,
  ResponseDecodeError,
} from './decode';
import type { MonitorClient, ReadResult } from './model';
import type { components } from '../../shared/api/public.generated';

const uncertain = {
  kind: 'uncertain',
  message:
    'The Monitor may have been created. Retrying can create a duplicate.',
} as const;
export function createMonitorClient(fetchImpl: typeof fetch): MonitorClient {
  async function read<T>(
    path: string,
    signal: AbortSignal,
    decode: (value: unknown) => T,
    empty?: () => T,
  ): Promise<ReadResult<T>> {
    try {
      const response = await requestJson(fetchImpl, path, {
        method: 'GET',
        signal,
      });
      if (response.status !== 200 && response.status !== 204)
        return {
          kind: 'error',
          error: {
            kind: 'http',
            status: response.status,
            message:
              response.status === 404
                ? 'Monitor not found.'
                : 'The read request failed.',
          },
        };
      return {
        kind: 'success',
        data:
          response.status === 204 && empty ? empty() : decode(response.value),
      };
    } catch (error) {
      return {
        kind: 'error',
        error: {
          kind:
            error instanceof ResponseDecodeError || error instanceof SyntaxError
              ? 'invalid_response'
              : 'transport',
          message: 'Unable to read a valid server response.',
        },
      };
    }
  }
  const resource = (id: string) => '/api/monitors/' + encodeURIComponent(id);
  return {
    async createMonitor(targetUrl, signal) {
      const body: components['schemas']['CreateMonitorRequest'] = { targetUrl };
      try {
        const response = await requestJson(fetchImpl, '/api/monitors', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
          signal,
        });
        if ([400, 403, 405, 413, 415, 422].includes(response.status))
          return {
            kind: 'rejected',
            message:
              'The request was rejected. Check the target URL and local access.',
          };
        if (response.status !== 201) return uncertain;
        return { kind: 'created', monitor: decodeMonitor(response.value) };
      } catch {
        return uncertain;
      }
    },
    getMonitor: (id, signal) => read(resource(id), signal, decodeMonitor),
    getLatestResult: (id, signal) =>
      read<import('./model').LatestResult | null>(
        resource(id) + '/latest-result',
        signal,
        decodeLatestResult,
        () => null,
      ),
    getAvailability: (id, signal) =>
      read(resource(id) + '/availability', signal, decodeAvailability),
  };
}
