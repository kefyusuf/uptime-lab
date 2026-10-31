import type { Monitor, MonitorClient } from '../../entities/monitor';
import { useEffect, useRef, useState, type FormEvent } from 'react';
export function CreateMonitorForm({
  client,
  onCreated,
}: {
  client: MonitorClient;
  onCreated: (monitor: Monitor) => void;
}) {
  const [target, setTarget] = useState(''),
    [pending, setPending] = useState(false),
    [error, setError] = useState<{
      kind: 'rejected' | 'uncertain';
      message: string;
    } | null>(null);
  const input = useRef<HTMLInputElement>(null),
    active = useRef<AbortController | null>(null);
  useEffect(
    () => () => {
      active.current?.abort();
      active.current = null;
    },
    [],
  );
  useEffect(() => {
    if (error?.kind === 'rejected') input.current?.focus();
  }, [error]);
  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (active.current) return;
    if (target.trim() === '') {
      setError({ kind: 'rejected', message: 'Enter a target URL.' });
      input.current?.focus();
      return;
    }
    const controller = new AbortController();
    active.current = controller;
    setPending(true);
    setError(null);
    try {
      const outcome = await client.createMonitor(target, controller.signal);
      if (active.current !== controller) return;
      if (outcome.kind === 'created') onCreated(outcome.monitor);
      else {
        setError(outcome);
        if (outcome.kind === 'rejected') input.current?.focus();
      }
    } catch {
      if (active.current === controller)
        setError({
          kind: 'uncertain',
          message:
            'The Monitor may have been created. Retrying can create a duplicate.',
        });
    } finally {
      if (active.current === controller) {
        active.current = null;
        setPending(false);
      }
    }
  }
  return (
    <form
      onSubmit={(event) => {
        void submit(event);
      }}
      noValidate
    >
      <label htmlFor="target-url">Target URL</label>
      <p id="target-help" className="field-help">
        Use an HTTP or HTTPS target. Registration does not confirm that it is
        reachable or safe to probe.
      </p>
      <input
        ref={input}
        id="target-url"
        name="targetUrl"
        type="text"
        placeholder="https://example.com/health"
        autoComplete="off"
        value={target}
        onChange={(event) => setTarget(event.target.value)}
        disabled={pending}
        aria-required="true"
        aria-invalid={error?.kind === 'rejected'}
        aria-describedby={'target-help' + (error ? ' create-error' : '')}
      />
      {error && (
        <p id="create-error" role="alert" className="message error">
          {error.message}
        </p>
      )}
      {pending && <p role="status">Registering monitor…</p>}
      <button type="submit" className="primary" disabled={pending}>
        {pending ? 'Creating…' : 'Create monitor'}
      </button>
    </form>
  );
}
