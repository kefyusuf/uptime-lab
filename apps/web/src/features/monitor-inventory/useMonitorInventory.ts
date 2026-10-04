import { useEffect, useRef, useState } from 'react';
import type {
  LoadState,
  MonitorClient,
  MonitorInventoryPage,
} from '../../entities/monitor';

export function useMonitorInventory(client: MonitorClient) {
  const [request, setRequest] = useState<{
    cursor: string | null;
    generation: number;
  }>({ cursor: null, generation: 0 });
  const [snapshot, setSnapshot] = useState<{
    client: MonitorClient;
    cursor: string | null;
    generation: number;
    state: LoadState<MonitorInventoryPage>;
  }>({ client, cursor: null, generation: -1, state: { kind: 'loading' } });
  const serial = useRef(0),
    active = useRef<AbortController | null>(null);
  useEffect(() => {
    const controller = new AbortController(),
      token = ++serial.current;
    active.current = controller;
    const current = () =>
      serial.current === token && !controller.signal.aborted;
    async function load() {
      let state: LoadState<MonitorInventoryPage>;
      try {
        state = await client.listMonitors(
          { limit: 20, cursor: request.cursor },
          controller.signal,
        );
      } catch {
        state = {
          kind: 'error',
          error: {
            kind: 'transport',
            message: 'Unable to read a valid server response.',
          },
        };
      }
      if (current()) setSnapshot({ client, ...request, state });
    }
    void load();
    return () => {
      controller.abort();
      if (active.current === controller) active.current = null;
    };
  }, [client, request]);
  const state: LoadState<MonitorInventoryPage> =
    snapshot.client === client &&
    snapshot.generation === request.generation &&
    snapshot.cursor === request.cursor
      ? snapshot.state
      : { kind: 'loading' };
  function load(cursor: string | null) {
    serial.current++;
    active.current?.abort();
    setRequest((previous) => ({ cursor, generation: previous.generation + 1 }));
  }
  return {
    state,
    cursor: request.cursor,
    next: () => {
      if (state.kind === 'success' && state.data.nextCursor !== null)
        load(state.data.nextCursor);
    },
    refresh: () => load(null),
    retry: () => {
      if (state.kind === 'error') load(request.cursor);
    },
  };
}
