import { render, screen } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { expect, it, vi } from 'vitest';
import type { MonitorClient } from '../../entities/monitor';
import { EditMonitorTargetControl } from './EditMonitorTargetControl';

const id = 'aa29443e-c597-4e7b-9202-a4762e0e04c0';
const monitor = {
  id,
  targetUrl: 'http://example.com/',
  createdAt: '2026-10-02T12:00:00Z',
};
const edited = { ...monitor, targetUrl: 'https://example.com/edited' };
function fakeClient(overrides: Record<string, unknown> = {}) {
  return {
    editTarget: vi.fn().mockResolvedValue({ kind: 'edited', data: edited }),
    getMonitor: vi.fn().mockResolvedValue({ kind: 'success', data: monitor }),
    ...overrides,
  } as unknown as MonitorClient;
}
function renderControl(client: MonitorClient, onEdited = vi.fn()) {
  render(
    <EditMonitorTargetControl
      id={id}
      client={client}
      targetUrl={monitor.targetUrl}
      onEdited={onEdited}
    />,
  );
  return onEdited;
}
it('hides the form until the operator starts editing', async () => {
  const client = fakeClient();
  renderControl(client);
  expect(screen.queryByLabelText('Target URL')).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Edit target' }));
  expect(screen.getByLabelText('Target URL')).toHaveValue(
    'http://example.com/',
  );
});
it('cancels back to the closed form', async () => {
  const client = fakeClient();
  renderControl(client);
  await userEvent.click(screen.getByRole('button', { name: 'Edit target' }));
  await userEvent.clear(screen.getByLabelText('Target URL'));
  await userEvent.type(screen.getByLabelText('Target URL'), 'changed');
  await userEvent.click(screen.getByRole('button', { name: 'Cancel' }));
  expect(screen.queryByLabelText('Target URL')).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Edit target' }));
  expect(screen.getByLabelText('Target URL')).toHaveValue(
    'http://example.com/',
  );
});
it('updates the displayed target through the parent refresh path', async () => {
  const client = fakeClient();
  const onEdited = renderControl(client);
  await userEvent.click(screen.getByRole('button', { name: 'Edit target' }));
  await userEvent.clear(screen.getByLabelText('Target URL'));
  await userEvent.type(
    screen.getByLabelText('Target URL'),
    'https://example.com/edited',
  );
  await userEvent.click(screen.getByRole('button', { name: 'Save target' }));
  await waitForSaved();
  expect(onEdited).toHaveBeenCalledTimes(1);
  expect(client.editTarget).toHaveBeenCalledTimes(1);
  expect(client.editTarget).toHaveBeenCalledWith(
    id,
    'https://example.com/edited',
    expect.any(AbortSignal),
  );
});
function waitForSaved() {
  return screen.findByText('Target updated.');
}
it('keeps the form editable with the rejection message', async () => {
  const client = fakeClient({
    editTarget: vi.fn().mockResolvedValue({
      kind: 'rejected',
      error: {
        kind: 'http',
        status: 422,
        message:
          'The target edit was rejected. Refresh the monitor before another change.',
      },
    }),
  });
  renderControl(client);
  await userEvent.click(screen.getByRole('button', { name: 'Edit target' }));
  await userEvent.clear(screen.getByLabelText('Target URL'));
  await userEvent.type(screen.getByLabelText('Target URL'), 'not-a-url');
  await userEvent.click(screen.getByRole('button', { name: 'Save target' }));
  expect(await screen.findByRole('alert')).toHaveTextContent(
    'The target edit was rejected.',
  );
  expect(screen.getByLabelText('Target URL')).toHaveValue('not-a-url');
});
it('resolves uncertain outcomes only through the explicit refresh', async () => {
  const client = fakeClient({
    editTarget: vi.fn().mockResolvedValue({
      kind: 'uncertain',
      error: {
        kind: 'transport',
        message: 'The target edit outcome is unknown. Refresh to check.',
      },
    }),
    getMonitor: vi.fn().mockResolvedValue({ kind: 'success', data: edited }),
  });
  const onEdited = renderControl(client);
  await userEvent.click(screen.getByRole('button', { name: 'Edit target' }));
  await userEvent.clear(screen.getByLabelText('Target URL'));
  await userEvent.type(
    screen.getByLabelText('Target URL'),
    'https://example.com/edited',
  );
  await userEvent.click(screen.getByRole('button', { name: 'Save target' }));
  const alert = await screen.findByRole('alert');
  expect(alert).toHaveTextContent('The target edit outcome is unknown.');
  expect(onEdited).not.toHaveBeenCalled();
  await userEvent.click(
    screen.getByRole('button', { name: 'Refresh monitor state' }),
  );
  await waitForSaved();
  expect(client.getMonitor).toHaveBeenCalledTimes(1);
  expect(onEdited).toHaveBeenCalledTimes(1);
});
