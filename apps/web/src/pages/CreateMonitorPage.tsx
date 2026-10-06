import type { MonitorClient } from '../entities/monitor';
import { useRef, useState, type FormEvent } from 'react';
import { CreateMonitorForm } from '../features/create-monitor';
import { MonitorInventory } from '../features/monitor-inventory';
export function CreateMonitorPage({
  client,
  onNavigate,
}: {
  client: MonitorClient;
  onNavigate: (path: string) => void;
}) {
  const [id, setId] = useState(''),
    [error, setError] = useState(false);
  const input = useRef<HTMLInputElement>(null);
  function open(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const value = id.trim();
    if (
      !/^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
        value,
      )
    ) {
      setError(true);
      input.current?.focus();
      return;
    }
    onNavigate('/monitors/' + value);
  }
  return (
    <div className="create-layout">
      <section className="panel">
        <p className="eyebrow">Start monitoring</p>
        <h2>Create monitor</h2>
        <p className="intro">
          Register a target and inspect its most recent assessment.
        </p>
        <CreateMonitorForm
          client={client}
          onCreated={(monitor) => onNavigate('/monitors/' + monitor.id)}
        />
      </section>
      <aside className="panel secondary">
        <h2>Return to a monitor</h2>
        <p>
          Open an existing monitor using its ID. Save its detail URL to return
          later.
        </p>
        <form onSubmit={open} noValidate>
          <label htmlFor="monitor-id">Monitor ID</label>
          <input
            ref={input}
            id="monitor-id"
            name="monitorId"
            autoComplete="off"
            value={id}
            onChange={(event) => {
              setId(event.target.value);
              setError(false);
            }}
            aria-invalid={error}
            aria-describedby={error ? 'open-error' : undefined}
          />
          {error && (
            <p id="open-error" role="alert" className="message error">
              Enter a valid Monitor ID.
            </p>
          )}
          <button type="submit">Open monitor</button>
        </form>
      </aside>
      <MonitorInventory client={client} onNavigate={onNavigate} />
    </div>
  );
}
