import { useEffect, useRef, useState } from 'react';
import {
  AvailabilityCard,
  LatestResultCard,
  UtcTime,
  type MonitorClient,
} from '../entities/monitor';
import { useMonitorReads } from '../features/refresh-monitor';
import { MonitorSchedulingControl } from '../features/set-monitor-scheduling';
export function MonitorDetailPage({
  id,
  client,
}: {
  id: string;
  client: MonitorClient;
}) {
  const reads = useMonitorReads(id, client),
    heading = useRef<HTMLHeadingElement>(null);
  useEffect(() => {
    heading.current?.focus();
  }, [id]);
  const monitor = reads.monitor.kind === 'success' ? reads.monitor.data : null;
  const [schedulingID, setSchedulingID] = useState<string | null>(null);
  if (monitor && schedulingID !== id) setSchedulingID(id);
  const error = reads.monitor.kind === 'error' ? reads.monitor.error : null;
  return (
    <div className="detail-layout">
      <section
        className="panel monitor-summary"
        aria-busy={reads.monitor.kind === 'loading'}
      >
        <div className="detail-heading">
          <div>
            <p className="eyebrow">Monitor workspace</p>
            <h2 ref={heading} tabIndex={-1}>
              Monitor detail
            </h2>
          </div>
          <button onClick={reads.refresh} aria-label="Refresh monitor">
            Refresh
          </button>
        </div>
        {monitor ? (
          <dl>
            <div>
              <dt>Target URL</dt>
              <dd data-testid="monitor-target">{monitor.targetUrl}</dd>
            </div>
            <div>
              <dt>Monitor ID</dt>
              <dd>
                <code data-testid="monitor-id">{monitor.id}</code>
              </dd>
            </div>
            <div>
              <dt>Registered at</dt>
              <dd>
                <UtcTime value={monitor.createdAt} label="Registration time" />
              </dd>
            </div>
          </dl>
        ) : error ? (
          <p role="alert" className="message error">
            {error.status === 400
              ? 'Invalid Monitor ID.'
              : error.status === 404
                ? 'Monitor not found.'
                : 'Monitor could not be read. Refresh to try again.'}
          </p>
        ) : (
          <p role="status">Reading monitor…</p>
        )}
      </section>
      {schedulingID === id && reads.monitor.kind !== 'error' && (
        <MonitorSchedulingControl key={id} id={id} client={client} />
      )}
      {monitor && (
        <div className="result-layout">
          <AvailabilityCard state={reads.availability} />
          <LatestResultCard state={reads.latest} />
        </div>
      )}
    </div>
  );
}
