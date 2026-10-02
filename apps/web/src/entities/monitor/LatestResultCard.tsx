import type { LatestResult, LoadState } from './model';
import { UtcTime } from './UtcTime';
const labels: Record<LatestResult['resultKind'], string> = {
  http_response: 'HTTP response',
  dns_error: 'DNS error',
  policy_rejected: 'Destination policy rejected',
  timeout: 'Probe timeout',
  connect_error: 'Connection error',
  tls_error: 'TLS error',
  protocol_error: 'Protocol error',
  internal_error: 'Execution error',
  worker_timeout: 'Worker timeout',
};
export function LatestResultCard({
  state,
}: {
  state: LoadState<LatestResult | null>;
}) {
  const value = state.kind === 'success' ? state.data : null;
  return (
    <section
      className="panel read-card"
      aria-labelledby="latest-title"
      aria-busy={state.kind === 'loading'}
    >
      <h3 id="latest-title">Latest execution</h3>
      {value ? (
        <>
          <p className="execution-kind">{labels[value.resultKind]}</p>
          <p className="snapshot-note">
            Execution fact; separate from the assessment
          </p>
          <dl>
            <div>
              <dt>Result kind</dt>
              <dd>
                <code data-testid="raw-result-kind">{value.resultKind}</code>
              </dd>
            </div>
            <div>
              <dt>Check ID</dt>
              <dd>
                <code data-testid="raw-check-id">{value.checkId}</code>
              </dd>
            </div>
            <div>
              <dt>Completed at</dt>
              <dd>
                <UtcTime
                  value={value.completedAt}
                  label="Execution completion time"
                  testId="raw-completed-at"
                />
              </dd>
            </div>
            {'httpStatus' in value && (
              <div>
                <dt>HTTP status</dt>
                <dd>{value.httpStatus}</dd>
              </div>
            )}
            {'durationMs' in value && (
              <div>
                <dt>Duration</dt>
                <dd>{value.durationMs} ms</dd>
              </div>
            )}
          </dl>
        </>
      ) : state.kind === 'success' ? (
        <p className="empty-state">No completed result yet</p>
      ) : state.kind === 'error' ? (
        <p role="alert" className="message error">
          Latest execution could not be read. Refresh to try again.
        </p>
      ) : (
        <p className="empty-state">
          {state.kind === 'loading'
            ? 'Reading execution…'
            : 'Execution not requested.'}
        </p>
      )}
    </section>
  );
}
