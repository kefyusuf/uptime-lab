import {
  mkdtempSync,
  mkdirSync,
  writeFileSync,
  rmSync,
  symlinkSync,
} from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { expect, it } from 'vitest';
import { loadAssets } from './assets.js';
function fixture(): string {
  const root = mkdtempSync(join(tmpdir(), 'uptime-assets-'));
  mkdirSync(join(root, '.vite'));
  mkdirSync(join(root, 'assets'));
  writeFileSync(join(root, 'index.html'), '<h1>Web</h1>');
  writeFileSync(join(root, 'assets/main.js'), 'console.log("built");');
  writeFileSync(join(root, 'assets/main.css'), 'body{color:black}');
  writeFileSync(
    join(root, '.vite/manifest.json'),
    JSON.stringify({
      'index.html': {
        file: 'assets/main.js',
        isEntry: true,
        css: ['assets/main.css'],
      },
    }),
  );
  return root;
}
it('loads only manifest assets and verifies completeness', () => {
  const root = fixture();
  try {
    const index = loadAssets(root);
    expect([...index.files.keys()].sort()).toEqual([
      '/assets/main.css',
      '/assets/main.js',
    ]);
    expect(index.ready()).toBe(true);
    rmSync(join(root, 'assets/main.css'));
    expect(index.ready()).toBe(false);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
it('rejects manifest traversal and source maps', () => {
  for (const file of ['../secret.js', 'assets/main.js.map']) {
    const root = fixture();
    try {
      writeFileSync(
        join(root, '.vite/manifest.json'),
        JSON.stringify({ 'index.html': { file, isEntry: true } }),
      );
      expect(() => loadAssets(root)).toThrow();
    } finally {
      rmSync(root, { recursive: true, force: true });
    }
  }
});
it('rejects missing build manifest', () => {
  const root = mkdtempSync(join(tmpdir(), 'uptime-assets-'));
  try {
    expect(() => loadAssets(root)).toThrow();
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
it('rejects a symlink asset escaping the build root', () => {
  const root = fixture(),
    outside = mkdtempSync(join(tmpdir(), 'uptime-assets-outside-'));
  try {
    writeFileSync(join(outside, 'secret.js'), 'secret');
    symlinkSync(outside, join(root, 'assets/link'), 'junction');
    writeFileSync(
      join(root, '.vite/manifest.json'),
      JSON.stringify({
        'index.html': { file: 'assets/link/secret.js', isEntry: true },
      }),
    );
    expect(() => loadAssets(root)).toThrow();
  } finally {
    rmSync(root, { recursive: true, force: true });
    rmSync(outside, { recursive: true, force: true });
  }
});
