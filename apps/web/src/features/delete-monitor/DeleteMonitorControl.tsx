import { useEffect } from 'react';
import type { MonitorClient } from '../../entities/monitor';
import { useMonitorDelete } from './useMonitorDelete';
export function DeleteMonitorControl({
  id,
  client,
  onDeleted,
}: {
  id: string;
  client: MonitorClient;
  onDeleted: () => void;
}) {
  const { state, requestDelete, confirmDelete, cancel, refresh } =
    useMonitorDelete(id, client);
  useEffect(() => {
    if (state.kind === 'deleted') onDeleted();
  }, [state, onDeleted]);
  return (
    <section className="panel" aria-label="Monitor deletion">
      <h3>Danger zone</h3>
      <p>
        Deleting removes the Monitor and all of its check evidence permanently.
      </p>
      {state.kind === 'deleted' ? (
        <p role="status">Monitor deleted.</p>
      ) : state.kind === 'confirming' ? (
        <>
          <p role="alert">
            This cannot be undone. The Monitor and its evidence are removed
            permanently.
          </p>
          <button data-testid="confirm-delete" onClick={confirmDelete}>
            Delete permanently
          </button>
          <button onClick={cancel}>Cancel</button>
        </>
      ) : (
        <>
          {state.kind === 'uncertain' && (
            <p role="alert">{state.error.message}</p>
          )}
          {state.kind === 'deleting' && (
            <p role="status">Deleting monitor…</p>
          )}
          <button
            disabled={state.kind === 'deleting'}
            onClick={requestDelete}
          >
            Delete monitor
          </button>
          {state.kind === 'uncertain' && (
            <button onClick={refresh}>Refresh monitor state</button>
          )}
        </>
      )}
    </section>
  );
}
