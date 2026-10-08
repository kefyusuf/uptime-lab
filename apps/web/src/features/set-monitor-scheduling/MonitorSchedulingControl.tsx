import type { MonitorClient } from '../../entities/monitor';
import { useMonitorScheduling } from './useMonitorScheduling';
export function MonitorSchedulingControl({
  id,
  client,
}: {
  id: string;
  client: MonitorClient;
}) {
  const { state, refresh, setState } = useMonitorScheduling(id, client);
  const ready = state.kind === 'ready',
    paused = ready && state.data.state === 'paused';
  return (
    <section
      className="panel"
      aria-label="Monitor scheduling"
      aria-busy={state.kind === 'loading' || state.kind === 'writing'}
    >
      <h3>Scheduling</h3>
      <p data-testid="scheduling-state" role="status">
        {ready
          ? paused
            ? 'Paused'
            : 'Active'
          : state.kind === 'writing'
            ? 'Writing scheduling state…'
            : state.kind === 'loading'
              ? 'Reading scheduling state…'
              : 'Unknown'}
      </p>
      <p>
        Pausing stops new claims. Already claimed checks may still complete.
      </p>
      <p>
        A read is only a current snapshot. After a lost write response, server
        work may still commit later. Refreshing or leaving this page does not
        cancel server work.
      </p>
      {(state.kind === 'uncertain' || state.kind === 'read-error') && (
        <p role="alert">{state.error.message}</p>
      )}
      <button
        disabled={!ready}
        onClick={() => setState(paused ? 'active' : 'paused')}
      >
        {paused ? 'Resume' : 'Pause'}
      </button>
      <button
        disabled={state.kind === 'writing' || state.kind === 'loading'}
        onClick={refresh}
      >
        Refresh scheduling state
      </button>
    </section>
  );
}
