import type { Availability, LoadState } from './model';
import { UtcTime } from './UtcTime';
const reasons: Record<Availability['reason'], string> = {
  successful_response: 'The target returned a successful response.',
  unexpected_http_status: 'The target returned an unexpected HTTP status.',
  probe_failure: 'The probe failed.',
  policy_rejected: 'Execution was blocked by destination policy.',
  execution_failure: 'Execution could not provide an assessment.',
  stale_result: 'The server reports that the result is too old.',
  future_result: 'The server reports a future completion time.',
  no_result: 'No completed result yet',
};
export function AvailabilityCard({
  state,
}: {
  state: LoadState<Availability>;
}) {
  const value = state.kind === 'success' ? state.data : null;
  return (
    <section
      className="panel read-card"
      aria-labelledby="availability-title"
      aria-busy={state.kind === 'loading'}
    >
      <h3 id="availability-title">Current availability</h3>
      {value ? (
        <>
          <p
            className={'assessment-status ' + value.status}
            data-testid="availability-status"
          >
            {value.status === 'available'
              ? 'Available'
              : value.status === 'unavailable'
                ? 'Unavailable'
                : 'Unknown'}
          </p>
          <p
            className="reason"
            data-testid="availability-reason"
            data-reason={value.reason}
          >
            {reasons[value.reason]}
          </p>
          <p className="snapshot-note">Snapshot; refresh to reassess</p>
          <dl>
            <div>
              <dt>Assessment at</dt>
              <dd>
                <UtcTime value={value.evaluatedAt} label="Assessment time" />
              </dd>
            </div>
            {'evidence' in value && (
              <>
                <div>
                  <dt>Evidence check ID</dt>
                  <dd>
                    <code data-testid="availability-check-id">
                      {value.evidence.checkId}
                    </code>
                  </dd>
                </div>
                <div>
                  <dt>Evidence completed at</dt>
                  <dd>
                    <UtcTime
                      value={value.evidence.completedAt}
                      label="Evidence completion time"
                      testId="availability-completed-at"
                    />
                  </dd>
                </div>
              </>
            )}
          </dl>
        </>
      ) : state.kind === 'error' ? (
        <p role="alert" className="message error">
          Availability could not be read. Refresh to try again.
        </p>
      ) : (
        <p className="empty-state">
          {state.kind === 'loading'
            ? 'Reading assessment…'
            : 'Assessment not requested.'}
        </p>
      )}
    </section>
  );
}
