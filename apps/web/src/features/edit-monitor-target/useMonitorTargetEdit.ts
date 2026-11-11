import { useCallback, useEffect, useRef, useState } from 'react';
import type { ClientError, MonitorClient } from '../../entities/monitor';

export type EditView =
  | { kind: 'idle' }
  | { kind: 'editing'; error: ClientError | null }
  | { kind: 'saving' }
  | { kind: 'saved' }
  | { kind: 'uncertain'; error: ClientError };
interface Session {
  id: string;
  client: MonitorClient;
  controller: AbortController | null;
  serial: number;
  view: EditView;
  live: boolean;
  pending: string | null;
}
const lostEdit: ClientError = {
  kind: 'transport',
  message: 'The target edit outcome is unknown. Refresh to check.',
};
export function useMonitorTargetEdit(id: string, client: MonitorClient) {
  const [snapshot, setSnapshot] = useState<{
    session: Session;
    view: EditView;
  } | null>(null);
  const active = useRef<Session | null>(null);
  const publish = useCallback((session: Session, view: EditView) => {
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
      pending: null,
    };
    active.current = session;
    return () => {
      session.live = false;
      session.serial++;
      session.controller?.abort();
      if (active.current === session) active.current = null;
    };
  }, [id, client]);
  const state: EditView =
    snapshot?.session.id === id &&
    snapshot.session.client === client &&
    snapshot.session.live
      ? snapshot.view
      : { kind: 'idle' };
  return {
    state,
    startEdit() {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        (session.view.kind !== 'idle' && session.view.kind !== 'saved')
      )
        return;
      publish(session, { kind: 'editing', error: null });
    },
    save(targetUrl: string) {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        session.view.kind !== 'editing'
      )
        return;
      publish(session, { kind: 'saving' });
      session.controller?.abort();
      const controller = new AbortController(),
        token = ++session.serial;
      session.controller = controller;
      session.pending = targetUrl;
      void (async () => {
        try {
          const result = await session.client.editTarget(
            session.id,
            targetUrl,
            controller.signal,
          );
          if (controller.signal.aborted || token !== session.serial) return;
          publish(
            session,
            result.kind === 'edited'
              ? { kind: 'saved' }
              : result.kind === 'rejected'
                ? { kind: 'editing', error: result.error }
                : { kind: 'uncertain', error: result.error },
          );
        } catch {
          if (!controller.signal.aborted && token === session.serial)
            publish(session, { kind: 'uncertain', error: lostEdit });
        }
      })();
    },
    cancel() {
      const session = active.current;
      if (
        !session ||
        session.id !== id ||
        session.client !== client ||
        session.view.kind !== 'editing'
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
            publish(session, { kind: 'uncertain', error: lostEdit });
            return;
          }
          publish(
            session,
            result.data.targetUrl === session.pending
              ? { kind: 'saved' }
              : { kind: 'idle' },
          );
        } catch {
          if (!controller.signal.aborted && token === session.serial)
            publish(session, { kind: 'uncertain', error: lostEdit });
        }
      })();
    },
  };
}
