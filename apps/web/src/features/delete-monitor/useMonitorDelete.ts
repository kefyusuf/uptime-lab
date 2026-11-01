import { useCallback, useEffect, useRef, useState } from 'react';
import type { ClientError, MonitorClient } from '../../entities/monitor';

export type DeleteView =
  | { kind: 'idle' }
  | { kind: 'confirming' }
  | { kind: 'deleting' }
  | { kind: 'uncertain'; error: ClientError }
  | { kind: 'deleted' };
interface Session {
  id: string;
  client: MonitorClient;
  controller: AbortController | null;
  serial: number;
  view: DeleteView;
  live: boolean;
}
const lostDelete: ClientError = {
  kind: 'transport',
  message: 'The deletion outcome is unknown. Refresh to check.',
};
export function useMonitorDelete(id: string, client: MonitorClient) {
  const [snapshot, setSnapshot] = useState<{
    session: Session;
    view: DeleteView;
  } | null>(null);
  const active = useRef<Session | null>(null);
  const publish = useCallback((session: Session, view: DeleteView) => {
    if (active.current !== session || !session.live) return;
    session.view = view;
    setSnapshot({ session, view });
  }, []);
  useEffect(() => {
    const session: Session = {
      id,
      client,
      controller: null,
      serial: 0,
      view: { kind: 'idle' },
      live: true,
    };
    active.current = session;
    return () => {
      session.live = false;
      session.serial++;
      session.controller?.abort();
      if (active.current === session) active.current = null;
    };
  }, [id, client]);
  const state: DeleteView =
    snapshot?.session.id === id &&
    snapshot.session.client === client &&
    snapshot.session.live
      ? snapshot.view
      : { kind: 'idle' };
  const confirm = useCallback(
    (session: Session) => {
      publish(session, { kind: 'deleting' });
      session.controller?.abort();
      const controller = new AbortController(),
        token = ++session.serial;
      session.controller = controller;
      void (async () => {
        try {
          const result = await session.client.deleteMonitor(
            session.id,
            controller.signal,
          );
          if (controller.signal.aborted || token !== session.serial) return;
          publish(
            session,
            result.kind === 'deleted'
              ? { kind: 'deleted' }
              : { kind: 'uncertain', error: result.error },
          );
        } catch {
          if (!controller.signal.aborted && token === session.serial)
            publish(session, { kind: 'uncertain', error: lostDelete });
        }
      })();
    },
    [publish],
  );
  return {
    state,
    requestDelete() {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        (session.view.kind !== 'idle' && session.view.kind !== 'uncertain')
      )
        return;
      publish(session, { kind: 'confirming' });
    },
    confirmDelete() {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        session.view.kind !== 'confirming'
      )
        return;
      confirm(session);
    },
    cancel() {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        session.view.kind !== 'confirming'
      )
        return;
      publish(session, { kind: 'idle' });
    },
    refresh() {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        session.view.kind !== 'uncertain'
      )
        return;
      session.controller?.abort();
      const controller = new AbortController(),
        token = ++session.serial;
      session.controller = controller;
      void (async () => {
        try {
          const result = await session.client.getMonitor(
            session.id,
            controller.signal,
          );
          if (controller.signal.aborted || token !== session.serial) return;
          if (result.kind === 'error') {
            if (result.error.status === 404) {
              publish(session, { kind: 'deleted' });
              return;
            }
            publish(session, { kind: 'uncertain', error: lostDelete });
            return;
          }
          publish(session, { kind: 'idle' });
        } catch {
          if (!controller.signal.aborted && token === session.serial)
            publish(session, { kind: 'uncertain', error: lostDelete });
        }
      })();
    },
  };
}
