import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import {
  createMonitorClient,
  type MonitorClient,
  type CreateOutcome,
} from '../../entities/monitor';
import { CreateMonitorForm } from './CreateMonitorForm';
const monitor = {
  id: 'aa29443e-c597-4e7b-9202-a4762e0e04c0',
  targetUrl: 'http://example.com/path with space',
  createdAt: '2026-10-02T12:00:00Z',
};
const client = (create: MonitorClient['createMonitor']): MonitorClient => ({
  getScheduling: vi.fn(),
  setScheduling: vi.fn(),
  createMonitor: create,
  listMonitors: vi.fn().mockResolvedValue({
    kind: 'success',
    data: { items: [], nextCursor: null },
  }),
  getMonitor: vi.fn(),
  getAvailability: vi.fn(),
  getLatestResult: vi.fn(),
});
async function enterAndSubmit() {
  const user = userEvent.setup();
  await user.type(screen.getByLabelText('Target URL'), monitor.targetUrl);
  await user.click(screen.getByRole('button', { name: 'Create monitor' }));
}
it('submits exact text and navigates after a validated result', async () => {
  const create = vi
      .fn<MonitorClient['createMonitor']>()
      .mockResolvedValue({ kind: 'created', monitor }),
    created = vi.fn();
  render(<CreateMonitorForm client={client(create)} onCreated={created} />);
  await enterAndSubmit();
  await waitFor(() => expect(created).toHaveBeenCalledWith(monitor));
  expect(create).toHaveBeenCalledWith(
    monitor.targetUrl,
    expect.any(AbortSignal),
  );
  expect(create).toHaveBeenCalledTimes(1);
});
it('focuses an empty target without submission', async () => {
  const create = vi.fn();
  render(<CreateMonitorForm client={client(create)} onCreated={vi.fn()} />);
  await userEvent.click(screen.getByRole('button', { name: 'Create monitor' }));
  expect(screen.getByLabelText('Target URL')).toHaveFocus();
  expect(screen.getByRole('alert')).toHaveTextContent('Enter a target URL');
  expect(create).not.toHaveBeenCalled();
});
it('locks duplicate submissions while pending', async () => {
  let finish: (value: CreateOutcome) => void = () => {};
  const create = vi.fn<MonitorClient['createMonitor']>().mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  render(<CreateMonitorForm client={client(create)} onCreated={vi.fn()} />);
  await enterAndSubmit();
  expect(screen.getByRole('button', { name: 'Creating…' })).toBeDisabled();
  fireEvent.submit(screen.getByLabelText('Target URL').closest('form')!);
  expect(create).toHaveBeenCalledTimes(1);
  finish({ kind: 'rejected', message: 'The request was rejected.' });
  await screen.findByRole('alert');
});
it('associates server rejection with input and focuses it', async () => {
  render(
    <CreateMonitorForm
      client={client(
        vi.fn().mockResolvedValue({
          kind: 'rejected',
          message: 'The request was rejected.',
        }),
      )}
      onCreated={vi.fn()}
    />,
  );
  await enterAndSubmit();
  await screen.findByRole('alert');
  expect(screen.getByLabelText('Target URL')).toHaveAttribute(
    'aria-invalid',
    'true',
  );
  expect(screen.getByLabelText('Target URL')).toHaveFocus();
});
it.each([500, 502, 504, 'disconnect', 'invalid201'] as const)(
  'warns on uncertain %j without retry',
  async (scenario) => {
    const fetcher = vi.fn<typeof fetch>();
    if (scenario === 'disconnect')
      fetcher.mockRejectedValue(new Error('secret'));
    else
      fetcher.mockResolvedValue(
        Response.json(
          {},
          { status: scenario === 'invalid201' ? 201 : scenario },
        ),
      );
    render(
      <CreateMonitorForm
        client={createMonitorClient(fetcher)}
        onCreated={vi.fn()}
      />,
    );
    await enterAndSubmit();
    expect(await screen.findByRole('alert')).toHaveTextContent(
      'The Monitor may have been created',
    );
    expect(screen.getByRole('alert')).toHaveTextContent('duplicate');
    expect(fetcher).toHaveBeenCalledTimes(1);
    await userEvent.click(
      screen.getByRole('button', { name: 'Create monitor' }),
    );
    await waitFor(() => expect(fetcher).toHaveBeenCalledTimes(2));
  },
);
it('aborts creation and ignores completion after unmount', async () => {
  let finish: (value: CreateOutcome) => void = () => {};
  const created = vi.fn();
  const create = vi.fn<MonitorClient['createMonitor']>().mockImplementation(
    () =>
      new Promise((resolve) => {
        finish = resolve;
      }),
  );
  const view = render(
    <CreateMonitorForm client={client(create)} onCreated={created} />,
  );
  await enterAndSubmit();
  view.unmount();
  expect(create.mock.calls[0][1].aborted).toBe(true);
  finish({ kind: 'created', monitor });
  await Promise.resolve();
  expect(created).not.toHaveBeenCalled();
});
