import type {
  Monitor,
  MonitorClient,
  Availability,
  LatestResult,
  LoadState,
} from '../../entities/monitor';
import type { ReadResult } from '../../entities/monitor';
import { useEffect, useRef, useState } from 'react';
interface Views {
  monitor: LoadState<Monitor>;
  availability: LoadState<Availability>;
  latest: LoadState<LatestResult | null>;
}
interface Snapshot extends Views {
  id: string;
  generation: number;
}
const loading = (): Views => ({
  monitor: { kind: 'loading' },
  availability: { kind: 'idle' },
  latest: { kind: 'idle' },
});
async function safelyRead<T>(
  read: () => Promise<ReadResult<T>>,
): Promise<ReadResult<T>> {
  try {
    return await read();
  } catch {
    return {
      kind: 'error',
      error: {
        kind: 'transport',
        message: 'Unable to read a valid server response.',
      },
    };
  }
}
export function useMonitorReads(
  id: string,
  client: MonitorClient,
): Views & { refresh: () => void } {
  const [generation, setGeneration] = useState(0),
    [snapshot, setSnapshot] = useState<Snapshot>(() => ({
      id,
      generation: 0,
      ...loading(),
    }));
  const serial = useRef(0),
    active = useRef<AbortController | null>(null);
  useEffect(() => {
    const controller = new AbortController(),
      token = ++serial.current;
    active.current = controller;
    const current = () =>
      serial.current === token && !controller.signal.aborted;
    async function load() {
      const monitor = await safelyRead(() =>
        client.getMonitor(id, controller.signal),
      );
      if (!current()) return;
      if (monitor.kind === 'error') {
        setSnapshot({
          id,
          generation,
          monitor,
          availability: { kind: 'idle' },
          latest: { kind: 'idle' },
        });
        return;
      }
      setSnapshot({
        id,
        generation,
        monitor,
        availability: { kind: 'loading' },
        latest: { kind: 'loading' },
      });
      void safelyRead(() => client.getAvailability(id, controller.signal)).then(
        (availability) => {
          if (current())
            setSnapshot((previous) =>
              previous.id === id && previous.generation === generation
                ? { ...previous, availability }
                : previous,
            );
        },
      );
      void safelyRead(() => client.getLatestResult(id, controller.signal)).then(
        (latest) => {
          if (current())
            setSnapshot((previous) =>
              previous.id === id && previous.generation === generation
                ? { ...previous, latest }
                : previous,
            );
        },
      );
    }
    void load();
    return () => {
      controller.abort();
      if (active.current === controller) active.current = null;
    };
  }, [id, client, generation]);
  const views =
    snapshot.id === id && snapshot.generation === generation
      ? snapshot
      : loading();
  return {
    monitor: views.monitor,
    availability: views.availability,
    latest: views.latest,
    refresh: () => {
      serial.current++;
      active.current?.abort();
      setGeneration((value) => value + 1);
    },
  };
}
