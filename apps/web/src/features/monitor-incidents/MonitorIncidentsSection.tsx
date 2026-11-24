import { UtcTime, type MonitorClient } from '../../entities/monitor';
import { useMonitorIncidents } from './useMonitorIncidents';
function windowSeconds(openedAt: string, resolvedAt: string): number {
  return Math.round(
    (new Date(resolvedAt).getTime() - new Date(openedAt).getTime()) / 1000,
  );
}
export function MonitorIncidentsSection({
  id,
  client,
}: {
  id: string;
  client: MonitorClient;
}) {
  const { state, refresh, next } = useMonitorIncidents(id, client);
  const ready = state.kind === 'ready';
  const nextCursor = ready ? state.data.nextCursor : null;
  return (
    <section
      className="panel"
      aria-label="Durable incident history"
      aria-busy={state.kind === 'loading'}
    >
      <h3>Incident history</h3>
      {state.kind === 'read-error' && <p role="alert">{state.error.message}</p>}
      {ready && state.data.items.length === 0 && (
        <p role="status" data-testid="incidents-empty">
          No incidents recorded.
        </p>
      )}
      {ready && state.data.items.length > 0 && (
        <ul data-testid="incidents-list">
          {state.data.items.map((entry) => (
            <li
              key={entry.incidentId}
              data-testid="incident-entry"
              data-incident-id={entry.incidentId}
            >
              <span data-testid="incident-reason">{entry.openingReason}</span>{' '}
              {entry.resolvedAt === undefined ? (
                <span data-testid="incident-open-badge">Open</span>
              ) : (
                <span data-testid="incident-duration">
                  {windowSeconds(entry.openedAt, entry.resolvedAt)}s
                </span>
              )}{' '}
              <UtcTime value={entry.openedAt} label="Opened at" />
              {entry.resolvedAt !== undefined && (
                <UtcTime value={entry.resolvedAt} label="Resolved at" />
              )}
            </li>
          ))}
        </ul>
      )}
      <button
        aria-label="Refresh incidents"
        disabled={state.kind === 'loading'}
        onClick={refresh}
      >
        Refresh incidents
      </button>
      <button
        aria-label="Next incident page"
        disabled={state.kind === 'loading' || !nextCursor}
        onClick={() => {
          if (nextCursor) next(nextCursor);
        }}
      >
        Next incident page
      </button>
    </section>
  );
}
