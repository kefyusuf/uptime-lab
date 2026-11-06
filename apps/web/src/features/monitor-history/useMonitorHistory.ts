import { useEffect, useRef, useState } from 'react';
import type {
  ClientError,
  MonitorClient,
  MonitorHistoryPage,
} from '../../entities/monitor';

export type HistoryView =
  | { kind: 'loading' }
  | { kind: 'ready'; data: MonitorHistoryPage }
  | { kind: 'read-error'; error: ClientError };
const failedRead: ClientError = {
  kind: 'transport',
  message: 'Unable to read a valid server response.',
};
export function useMonitorHistory(id: string, client: MonitorClient) {
  const [request, setRequest] = useState<{
    cursor: string | null;
    generation: number;
  }>({ cursor: null, generation: 0 });
  const [snapshot, setSnapshot] = useState<{
    client: MonitorClient;
    id: string;
    cursor: string | null;
    generation: number;
    state: HistoryView;
  }>({
    client,
    id,
    cursor: null,
    generation: -1,
    state: { kind: 'loading' },
  });
  const serial = useRef(0),
    active = useRef<AbortController | null>(null);
  useEffect(() => {
    const controller = new AbortController(),
      token = ++serial.current;
    active.current = controller;
    const current = () =>
      serial.current === token && !controller.signal.aborted;
    async function load() {
      let state: HistoryView;
      try {
        const result = await client.getHistory(
          id,
          request.cursor,
          controller.signal,
        );
        state =
          result.kind === 'success'
            ? { kind: 'ready', data: result.data }
            : { kind: 'read-error', error: result.error };
      } catch {
        state = { kind: 'read-error', error: failedRead };
      }
      if (current())
        setSnapshot({
          client,
          id,
          cursor: request.cursor,
          generation: request.generation,
          state,
        });
    }
    void load();
    return () => {
      controller.abort();
      if (active.current === controller) active.current = null;
    };
  }, [client, id, request]);
  const state: HistoryView =
    snapshot.client === client &&
    snapshot.id === id &&
    snapshot.generation === request.generation &&
    snapshot.cursor === request.cursor
      ? snapshot.state
      : { kind: 'loading' };
  function load(cursor: string | null) {
    serial.current++;
    active.current?.abort();
    setRequest((previous) => ({
      cursor,
      generation: previous.generation + 1,
    }));
  }
  return {
    state,
    refresh() {
      if (state.kind !== 'loading') load(null);
    },
    next(cursor: string) {
      if (state.kind !== 'ready' || state.data.nextCursor !== cursor) return;
      load(cursor);
    },
  };
}
