import type { MonitorClient } from '../../entities/monitor';
import { UtcTime } from '../../entities/monitor';
import { useMonitorInventory } from './useMonitorInventory';

export function MonitorInventory({
  client,
  onNavigate,
}: {
  client: MonitorClient;
  onNavigate: (path: string) => void;
}) {
  const { state, cursor, next, refresh, retry } = useMonitorInventory(client);
  return (
    <section
      className="panel inventory-panel"
      aria-labelledby="inventory-heading"
    >
      <h2 id="inventory-heading">Registered monitors</h2>
      <p>Find a registered monitor and open its detail.</p>
      <div className="inventory-actions">
        <button type="button" onClick={refresh}>
          Refresh list
        </button>
        {state.kind === 'success' && state.data.nextCursor !== null && (
          <button type="button" onClick={next}>
            Next page
          </button>
        )}
      </div>
      {state.kind === 'loading' && (
        <p role="status">Loading registered monitors…</p>
      )}
      {state.kind === 'error' && (
        <>
          <p role="alert" className="message error">
            {state.error.message}
          </p>
          <button type="button" onClick={retry}>
            Retry list
          </button>
        </>
      )}
      {state.kind === 'success' &&
        (state.data.items.length === 0 ? (
          <p className="empty-state">
            {cursor === null
              ? 'No monitors registered yet.'
              : 'No more monitors on this page. Refresh the list to start again.'}
          </p>
        ) : (
          <ul
            className="inventory-list"
            aria-label="Registered monitors"
            data-testid="inventory-list"
          >
            {state.data.items.map((monitor) => (
              <li key={monitor.id} data-testid="inventory-row">
                <p className="inventory-target">{monitor.targetUrl}</p>
                <p className="inventory-id">{monitor.id}</p>
                <UtcTime
                  value={monitor.createdAt}
                  label="Monitor creation time"
                />
                <a
                  href={'/monitors/' + monitor.id}
                  aria-label={'Open monitor ' + monitor.id}
                  onClick={(event) => {
                    if (
                      event.button === 0 &&
                      !event.ctrlKey &&
                      !event.metaKey &&
                      !event.altKey &&
                      !event.shiftKey
                    ) {
                      event.preventDefault();
                      onNavigate('/monitors/' + monitor.id);
                    }
                  }}
                >
                  Open detail
                </a>
              </li>
            ))}
          </ul>
        ))}
    </section>
  );
}
