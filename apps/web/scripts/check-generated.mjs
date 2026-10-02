import { mkdtempSync, readFileSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';
const directory = mkdtempSync(join(tmpdir(), 'uptime-web-types-'));
try {
  const output = join(directory, 'public.generated.ts');
  const command = spawnSync(
    process.execPath,
    [
      resolve('node_modules/openapi-typescript/bin/cli.js'),
      '../../contracts/openapi/public.yaml',
      '-o',
      output,
    ],
    { encoding: 'utf8' },
  );
  if (command.status !== 0) throw new Error('Public type generation failed.');
  if (
    readFileSync(output, 'utf8') !==
    readFileSync('src/shared/api/public.generated.ts', 'utf8')
  )
    throw new Error(
      'Generated public API types drifted. Run npm run generate:api.',
    );
  console.log('Generated public API types match.');
} finally {
  rmSync(directory, { recursive: true, force: true });
}
