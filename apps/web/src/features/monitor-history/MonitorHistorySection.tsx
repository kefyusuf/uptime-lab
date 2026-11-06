import { UtcTime, type MonitorClient } from '../../entities/monitor';
import { useMonitorHistory } from './useMonitorHistory';
export function MonitorHistorySection({
  id,
  client,
}: {
  id: string;
  client: MonitorClient;
}) {
  const { state, refresh, next } = useMonitorHistory(id, client);
  const ready = state.kind === 'ready';
  const nextCursor = ready ? state.data.nextCursor : null;
  return (
    <section
      className="panel"
      aria-label="Check-run history"
      aria-busy={state.kind === 'loading'}
    >
      <h3>Check-run history</h3>
      {state.kind === 'read-error' && <p role="alert">{state.error.message}</p>}
      {ready && state.data.items.length === 0 && (
        <p role="status" data-testid="history-empty">
          No completed checks yet.
        </p>
      )}
      {ready && state.data.items.length > 0 && (
        <ul data-testid="history-list">
          {state.data.items.map((entry) => (
            <li key={entry.checkId} data-testid="history-entry">
              <span data-testid="history-kind">{entry.resultKind}</span>{' '}
              <UtcTime value={entry.completedAt} label="Completion time" />
            </li>
          ))}
        </ul>
      )}
      <button
        aria-label="Refresh history"
        disabled={state.kind === 'loading'}
        onClick={refresh}
      >
        Refresh history
      </button>
      <button
        aria-label="Next history page"
        disabled={state.kind === 'loading' || !nextCursor}
        onClick={() => {
          if (nextCursor) next(nextCursor);
        }}
      >
        Next history page
      </button>
    </section>
  );
}
