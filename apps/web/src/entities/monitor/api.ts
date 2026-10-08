import { requestJson } from '../../shared/api/http';
import { decodeMonitorScheduling } from './scheduling';
import { decodeInventoryPage, decodeInventoryProblem } from './inventory';
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
    getScheduling: (id, signal) =>
      read(resource(id) + '/scheduling', signal, decodeMonitorScheduling),
    async setScheduling(id, state, signal) {
      try {
        const response = await requestJson(
          fetchImpl,
          resource(id) + '/scheduling',
          {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ state }),
            signal,
          },
        );
        if ([400, 404, 413, 415].includes(response.status))
          return {
            kind: 'rejected',
            error: {
              kind: 'http',
              status: response.status,
              message:
                'Scheduling request rejected. Refresh scheduling state before another change.',
            },
          };
        if (response.status !== 200)
          return {
            kind: 'uncertain',
            error: {
              kind: 'http',
              status: response.status,
              message:
                'The scheduling write may have completed. Refresh scheduling state.',
            },
          };
        const data = decodeMonitorScheduling(response.value);
        if (data.state !== state) throw new ResponseDecodeError();
        return { kind: 'confirmed', data };
      } catch {
        return {
          kind: 'uncertain',
          error: {
            kind: 'transport',
            message:
              'The scheduling write may have completed. Refresh scheduling state.',
          },
        };
      }
    },
    async listMonitors(input, signal) {
      if (
        !Number.isInteger(input.limit) ||
        input.limit < 1 ||
        input.limit > 50 ||
        (input.cursor !== null &&
          (typeof input.cursor !== 'string' ||
            input.cursor.length !== 88 ||
            !/^[A-Za-z0-9_-]{88}$/.test(input.cursor)))
      )
        return {
          kind: 'error',
          error: {
            kind: 'invalid_response',
            message: 'The inventory query is invalid.',
          },
        };
      const path =
        '/api/monitors?limit=' +
        input.limit +
        (input.cursor === null ? '' : '&cursor=' + input.cursor);
      try {
        const response = await requestJson(fetchImpl, path, {
          method: 'GET',
          signal,
        });
        if (response.status === 204) throw new ResponseDecodeError();
        if (response.status !== 200)
          return {
            kind: 'error',
            error: {
              kind: 'http',
              status: response.status,
              message:
                response.status === 500 &&
                decodeInventoryProblem(response.value) === 'oversized'
                  ? 'A registered monitor exceeds the inventory response limit.'
                  : 'The inventory read request failed.',
            },
          };
        return {
          kind: 'success',
          data: decodeInventoryPage(response.value, input.limit),
        };
      } catch (error) {
        return {
          kind: 'error',
          error: {
            kind:
              error instanceof ResponseDecodeError ||
              error instanceof SyntaxError
                ? 'invalid_response'
                : 'transport',
            message: 'Unable to read a valid server response.',
          },
        };
      }
    },
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
