export interface Monitor {
  id: string;
  targetUrl: string;
  createdAt: string;
}
export interface MonitorInventoryPage {
  items: Monitor[];
  nextCursor: string | null;
}
export interface MonitorScheduling {
  state: 'active' | 'paused';
}
export type SchedulingOutcome =
  | { kind: 'confirmed'; data: MonitorScheduling }
  | { kind: 'rejected'; error: ClientError }
  | { kind: 'uncertain'; error: ClientError };
export interface Evidence {
  checkId: string;
  completedAt: string;
}
export type FailureKind =
  | 'dns_error'
  | 'policy_rejected'
  | 'timeout'
  | 'connect_error'
  | 'tls_error'
  | 'protocol_error'
  | 'internal_error';
export type LatestResult = Evidence &
  (
    | { resultKind: 'http_response'; httpStatus: number; durationMs: number }
    | { resultKind: FailureKind; durationMs: number }
    | { resultKind: 'worker_timeout' }
  );
export type Availability =
  | {
      status: 'available';
      reason: 'successful_response';
      evaluatedAt: string;
      evidence: Evidence;
    }
  | {
      status: 'unavailable';
      reason: 'unexpected_http_status' | 'probe_failure';
      evaluatedAt: string;
      evidence: Evidence;
    }
  | {
      status: 'unknown';
      reason:
        | 'policy_rejected'
        | 'execution_failure'
        | 'stale_result'
        | 'future_result';
      evaluatedAt: string;
      evidence: Evidence;
    }
  | { status: 'unknown'; reason: 'no_result'; evaluatedAt: string };
export interface ClientError {
  kind: 'http' | 'transport' | 'invalid_response';
  status?: number;
  message: string;
}
export type ReadResult<T> =
  { kind: 'success'; data: T } | { kind: 'error'; error: ClientError };
export type CreateOutcome =
  | { kind: 'created'; monitor: Monitor }
  | { kind: 'rejected'; message: string }
  | { kind: 'uncertain'; message: string };
export interface MonitorClient {
  getScheduling(
    id: string,
    signal: AbortSignal,
  ): Promise<ReadResult<MonitorScheduling>>;
  setScheduling(
    id: string,
    state: MonitorScheduling['state'],
    signal: AbortSignal,
  ): Promise<SchedulingOutcome>;
  listMonitors(
    input: { limit: number; cursor: string | null },
    signal: AbortSignal,
  ): Promise<ReadResult<MonitorInventoryPage>>;
  createMonitor(targetUrl: string, signal: AbortSignal): Promise<CreateOutcome>;
  getMonitor(id: string, signal: AbortSignal): Promise<ReadResult<Monitor>>;
  getLatestResult(
    id: string,
    signal: AbortSignal,
  ): Promise<ReadResult<LatestResult | null>>;
  getAvailability(
    id: string,
    signal: AbortSignal,
  ): Promise<ReadResult<Availability>>;
}
export type LoadState<T> =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | { kind: 'success'; data: T }
  | { kind: 'error'; error: ClientError };
