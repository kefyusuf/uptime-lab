import { useEffect, useState } from 'react';
import type { MonitorClient } from '../../entities/monitor';
import { useMonitorTargetEdit } from './useMonitorTargetEdit';

export function EditMonitorTargetControl({
  id,
  client,
  targetUrl,
  onEdited,
}: {
  id: string;
  client: MonitorClient;
  targetUrl: string;
  onEdited: () => void;
}) {
  const { state, startEdit, save, cancel, refresh } = useMonitorTargetEdit(
    id,
    client,
  );
  const [draft, setDraft] = useState(targetUrl);
  useEffect(() => {
    if (state.kind === 'saved') onEdited();
  }, [state, onEdited]);
  if (state.kind === 'idle') {
    return (
      <section className="panel" aria-label="Monitor target edit">
        <h3>Edit target</h3>
        <p>
          Editing replaces the probe target on the same Monitor identity;
          registration time and recorded evidence stay untouched.
        </p>
        <button
          data-testid="edit-target"
          onClick={() => {
            setDraft(targetUrl);
            startEdit();
          }}
        >
          Edit target
        </button>
      </section>
    );
  }
  return (
    <section className="panel" aria-label="Monitor target edit">
      <h3>Edit target</h3>
      {state.kind === 'editing' && (
        <form
          onSubmit={(event) => {
            event.preventDefault();
            save(draft);
          }}
        >
          <label htmlFor="edit-target-url">Target URL</label>
          <input
            id="edit-target-url"
            data-testid="edit-target-input"
            value={draft}
            onChange={(event) => setDraft(event.target.value)}
            autoComplete="off"
            spellCheck={false}
          />
          {state.error && <p role="alert">{state.error.message}</p>}
          <button data-testid="save-target" type="submit">
            Save target
          </button>
          <button type="button" onClick={cancel}>
            Cancel
          </button>
        </form>
      )}
      {state.kind === 'saving' && <p role="status">Saving target…</p>}
      {state.kind === 'saved' && <p role="status">Target updated.</p>}
      {state.kind === 'uncertain' && (
        <>
          <p role="alert">{state.error.message}</p>
          <button onClick={refresh}>Refresh monitor state</button>
        </>
      )}
    </section>
  );
}
