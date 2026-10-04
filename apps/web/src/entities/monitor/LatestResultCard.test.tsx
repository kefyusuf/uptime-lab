import { readFileSync, readdirSync } from 'node:fs';
import { resolve } from 'node:path';
import { render, screen } from '@testing-library/react';
import { expect, it } from 'vitest';
import { decodeLatestResult } from './decode';
import { LatestResultCard } from './LatestResultCard';
const directory = resolve('../../contracts/fixtures/public');
it.each(
  readdirSync(directory).filter((name) => name.startsWith('latest-result-')),
)('presents execution fact %s', (name) => {
  const value = decodeLatestResult(
    JSON.parse(readFileSync(resolve(directory, name), 'utf8')),
  );
  render(<LatestResultCard state={{ kind: 'success', data: value }} />);
  expect(screen.getByText(value.checkId)).toBeInTheDocument();
  expect(screen.getByText('Completed at')).toBeInTheDocument();
  if ('durationMs' in value)
    expect(screen.getByText(value.durationMs + ' ms')).toBeInTheDocument();
  else expect(screen.queryByText('Duration')).not.toBeInTheDocument();
  expect(
    screen.queryByText('Available', { exact: true }),
  ).not.toBeInTheDocument();
});
it('displays raw204 as no completed result', () => {
  render(<LatestResultCard state={{ kind: 'success', data: null }} />);
  expect(screen.getByText('No completed result yet')).toBeInTheDocument();
  expect(screen.queryByText('Check ID')).not.toBeInTheDocument();
});
