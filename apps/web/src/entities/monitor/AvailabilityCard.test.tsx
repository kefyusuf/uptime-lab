import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import { decodeAvailability } from './decode';
import { AvailabilityCard } from './AvailabilityCard';
const directory = resolve('../../contracts/fixtures/public');
it.each(
  readdirSync(directory).filter((name) => name.startsWith('availability-')),
)('presents Go fixture %s without classifying it', (name) => {
  const value = decodeAvailability(
    JSON.parse(readFileSync(resolve(directory, name), 'utf8')),
  );
  render(<AvailabilityCard state={{ kind: 'success', data: value }} />);
  expect(
    screen.getByText(
      value.status === 'available'
        ? 'Available'
        : value.status === 'unavailable'
          ? 'Unavailable'
          : 'Unknown',
      { exact: true },
    ),
  ).toBeInTheDocument();
  expect(screen.getByText('Snapshot; refresh to reassess')).toBeInTheDocument();
  expect(screen.getByText('Assessment at')).toBeInTheDocument();
  expect(screen.getByLabelText('Assessment time')).toHaveAttribute(
    'datetime',
    value.evaluatedAt,
  );
  if ('evidence' in value)
    expect(screen.getByText(value.evidence.checkId)).toBeInTheDocument();
  else expect(screen.queryByText('Evidence check ID')).not.toBeInTheDocument();
});
it('does not show a healthy badge on read failure', () => {
  render(
    <AvailabilityCard
      state={{
        kind: 'error',
        error: { kind: 'transport', message: 'Read failed.' },
      }}
    />,
  );
  expect(
    screen.queryByText('Available', { exact: true }),
  ).not.toBeInTheDocument();
  expect(screen.getByRole('alert')).toHaveTextContent('could not be read');
});
it('cardRendersRecovering', () => {
  const value = decodeAvailability(
    JSON.parse(
      readFileSync(
        resolve(directory, 'availability-unavailable-recovering.json'),
        'utf8',
      ),
    ),
  );
  render(<AvailabilityCard state={{ kind: 'success', data: value }} />);
  expect(screen.getByText('Unavailable', { exact: true })).toBeInTheDocument();
  expect(screen.getByTestId('availability-reason')).toHaveAttribute(
    'data-reason',
    'recovering',
  );
  expect(
    screen.getByText(
      'The target is succeeding again but has not yet reached the recovery threshold.',
    ),
  ).toBeInTheDocument();
});
it('cardRendersAwaitingConfirmation', () => {
  const value = decodeAvailability(
    JSON.parse(
      readFileSync(
        resolve(directory, 'availability-unknown-awaiting-confirmation.json'),
        'utf8',
      ),
    ),
  );
  render(<AvailabilityCard state={{ kind: 'success', data: value }} />);
  expect(screen.getByText('Unknown', { exact: true })).toBeInTheDocument();
  expect(screen.getByTestId('availability-reason')).toHaveAttribute(
    'data-reason',
    'awaiting_confirmation',
  );
  expect(
    screen.getByText(
      'Failures have not yet reached the confirmation threshold.',
    ),
  ).toBeInTheDocument();
});
