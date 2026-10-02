import { useMemo, useSyncExternalStore } from 'react';
import { createMonitorClient, type MonitorClient } from '../entities/monitor';
import { CreateMonitorPage } from '../pages/CreateMonitorPage';
import { navigate, parsePage, subscribeNavigation } from './router';
export function App({
  client: providedClient,
}: { client?: MonitorClient } = {}) {
  const client = useMemo(
    () =>
      providedClient || createMonitorClient(globalThis.fetch.bind(globalThis)),
    [providedClient],
  );
  const path = useSyncExternalStore(
      subscribeNavigation,
      () => window.location.pathname,
    ),
    page = parsePage(path);
  return (
    <div className="shell">
      <a className="skip-link" href="#main">
        Skip to content
      </a>
      <header className="site-header">
        <a
          className="brand"
          href="/"
          onClick={(event) => {
            if (
              !event.ctrlKey &&
              !event.metaKey &&
              !event.shiftKey &&
              !event.altKey
            ) {
              event.preventDefault();
              navigate('/');
            }
          }}
        >
          <span className="brand-mark" aria-hidden="true">
            ◉
          </span>
          <h1>uptime-lab</h1>
        </a>
        <span className="workspace-label">Local workspace</span>
      </header>
      <main id="main">
        {page.kind === 'create' ? (
          <CreateMonitorPage client={client} onNavigate={navigate} />
        ) : page.kind === 'detail' ? (
          <section className="panel">
            <h2>Monitor detail</h2>
            <p>{page.id}</p>
          </section>
        ) : (
          <section className="panel">
            <h2>Page not found</h2>
            <p>Return to the registration screen to open a monitor.</p>
          </section>
        )}
      </main>
      <footer>
        Assessments are snapshots. Refresh a monitor to see a new assessment.
      </footer>
    </div>
  );
}
