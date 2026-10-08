import { useCallback, useEffect, useRef, useState } from 'react';
import type {
  ClientError,
  MonitorClient,
  MonitorScheduling,
} from '../../entities/monitor';

export type SchedulingView =
  | { kind: 'loading' }
  | { kind: 'ready'; data: MonitorScheduling }
  | { kind: 'writing' }
  | { kind: 'read-error' | 'uncertain'; error: ClientError };
interface Session {
  id: string;
  client: MonitorClient;
  controller: AbortController | null;
  serial: number;
  view: SchedulingView;
  live: boolean;
}
const failedRead: ClientError = {
  kind: 'transport',
  message: 'Unable to read scheduling state.',
};
export function useMonitorScheduling(id: string, client: MonitorClient) {
  const [snapshot, setSnapshot] = useState<{
    session: Session;
    view: SchedulingView;
  } | null>(null);
  const active = useRef<Session | null>(null);
  const publish = useCallback((session: Session, view: SchedulingView) => {
    if (active.current !== session || !session.live) return;
    session.view = view;
    setSnapshot({ session, view });
  }, []);
  const read = useCallback(
    async (session: Session) => {
      session.controller?.abort();
      const controller = new AbortController(),
        token = ++session.serial;
      session.controller = controller;
      try {
        const result = await session.client.getScheduling(
          session.id,
          controller.signal,
        );
        if (controller.signal.aborted || token !== session.serial) return;
        publish(
          session,
          result.kind === 'success'
            ? { kind: 'ready', data: result.data }
            : { kind: 'read-error', error: result.error },
        );
      } catch {
        if (!controller.signal.aborted && token === session.serial)
          publish(session, { kind: 'read-error', error: failedRead });
      }
    },
    [publish],
  );
  useEffect(() => {
    const session: Session = {
      id,
      client,
      controller: null,
      serial: 0,
      view: { kind: 'loading' },
      live: true,
    };
    active.current = session;
    void read(session);
    return () => {
      session.live = false;
      session.serial++;
      session.controller?.abort();
      if (active.current === session) active.current = null;
    };
  }, [id, client, read]);
  const state: SchedulingView =
    snapshot?.session.id === id &&
    snapshot.session.client === client &&
    snapshot.session.live
      ? snapshot.view
      : { kind: 'loading' };
  return {
    state,
    refresh() {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        session.view.kind === 'writing'
      )
        return;
      publish(session, { kind: 'loading' });
      void read(session);
    },
    setState(desired: MonitorScheduling['state']) {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        session.view.kind !== 'ready'
      )
        return;
      publish(session, { kind: 'writing' });
      session.controller?.abort();
      const controller = new AbortController(),
        token = ++session.serial;
      session.controller = controller;
      void (async () => {
        try {
          const result = await session.client.setScheduling(
            session.id,
            desired,
            controller.signal,
          );
          if (controller.signal.aborted || token !== session.serial) return;
          publish(
            session,
            result.kind === 'confirmed'
              ? { kind: 'ready', data: result.data }
              : {
                  kind: result.kind === 'rejected' ? 'read-error' : 'uncertain',
                  error: result.error,
                },
          );
        } catch {
          if (!controller.signal.aborted && token === session.serial)
            publish(session, {
              kind: 'uncertain',
              error: {
                kind: 'transport',
                message:
                  'The scheduling write may have completed. Refresh scheduling state.',
              },
            });
        }
      })();
    },
  };
}
