export async function requestJson(
  fetchImpl: typeof fetch,
  input: string,
  init: RequestInit,
): Promise<{ status: number; value: unknown }> {
  const controller = new AbortController();
  let rejectAbort: (reason: Error) => void = () => {};
  const aborted = new Promise<never>((_resolve, reject) => {
    rejectAbort = reject;
  });
  const cancel = () => {
    controller.abort();
    rejectAbort(new Error('Request cancelled.'));
  };
  init.signal?.addEventListener('abort', cancel, { once: true });
  const timer = setTimeout(cancel, 12000);
  try {
    if (init.signal?.aborted) cancel();
    const request = (async () => {
      const response = await fetchImpl(input, {
        ...init,
        signal: controller.signal,
        cache: 'no-store',
        redirect: 'error',
        credentials: 'omit',
      });
      return {
        status: response.status,
        value:
          response.status === 204 ? null : ((await response.json()) as unknown),
      };
    })();
    return await Promise.race([request, aborted]);
  } finally {
    clearTimeout(timer);
    init.signal?.removeEventListener('abort', cancel);
  }
}
