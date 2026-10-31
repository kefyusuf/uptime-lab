import type { IncomingMessage, ServerResponse } from 'node:http';
import { readFileSync, realpathSync, statSync } from 'node:fs';
import { extname, isAbsolute, relative, resolve, sep } from 'node:path';
export interface AssetIndex {
  root: string;
  ready: () => boolean;
  files: Map<string, { path: string; type: string }>;
  html: string;
}
const types: Record<string, string> = {
  '.js': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.jpg': 'image/jpeg',
  '.jpeg': 'image/jpeg',
  '.webp': 'image/webp',
  '.gif': 'image/gif',
  '.ico': 'image/x-icon',
  '.woff': 'font/woff',
  '.woff2': 'font/woff2',
};
function contained(root: string, file: string): string {
  if (
    isAbsolute(file) ||
    file.includes('\\') ||
    file.includes('%') ||
    file.split('/').some((part) => part === '..' || part === '.' || part === '')
  )
    throw Error('Invalid build asset.');
  const path = realpathSync(resolve(root, file)),
    distance = relative(root, path);
  if (
    distance.startsWith('..' + sep) ||
    isAbsolute(distance) ||
    !statSync(path).isFile()
  )
    throw Error('Invalid build asset.');
  return path;
}
export function loadAssets(distDir: string): AssetIndex {
  const root = realpathSync(distDir),
    html = contained(root, 'index.html');
  const manifest: unknown = JSON.parse(
    readFileSync(contained(root, '.vite/manifest.json'), 'utf8'),
  );
  if (!manifest || typeof manifest !== 'object' || Array.isArray(manifest))
    throw Error('Invalid build manifest.');
  const files = new Map<string, { path: string; type: string }>();
  const add = (file: unknown) => {
    if (typeof file !== 'string' || !types[extname(file)])
      throw Error('Invalid build asset.');
    files.set('/' + file, {
      path: contained(root, file),
      type: types[extname(file)],
    });
  };
  let entry = false;
  for (const value of Object.values(manifest)) {
    if (!value || typeof value !== 'object' || Array.isArray(value))
      throw Error('Invalid build manifest.');
    const item = value as Record<string, unknown>;
    add(item.file);
    if (item.isEntry === true) entry = true;
    for (const key of ['css', 'assets'])
      if (item[key] !== undefined) {
        if (!Array.isArray(item[key])) throw Error('Invalid build manifest.');
        for (const file of item[key]) add(file);
      }
  }
  if (!entry || files.size === 0) throw Error('Incomplete Web build.');
  const ready = () => {
    try {
      contained(root, 'index.html');
      contained(root, '.vite/manifest.json');
      for (const path of files.keys()) contained(root, path.slice(1));
      return true;
    } catch {
      return false;
    }
  };
  return { root, html, files, ready };
}
export function serveAsset(
  req: IncomingMessage,
  res: ServerResponse,
  index: AssetIndex,
): boolean {
  const path = req.url || '';
  if (
    /[%\\?#]/.test(path) ||
    path.includes('//') ||
    path.split('/').some((part) => part === '.' || part === '..')
  )
    return false;
  const shell = path === '/' || /^\/monitors\/[A-Za-z0-9-]+$/.test(path);
  const asset = index.files.get(path);
  if (!shell && !asset) return false;
  if (req.method !== 'GET' && req.method !== 'HEAD') {
    res.writeHead(405, { Allow: 'GET, HEAD', 'Cache-Control': 'no-store' });
    res.end();
    return true;
  }
  try {
    const file = contained(index.root, shell ? 'index.html' : path.slice(1));
    const body = readFileSync(file);
    res.writeHead(200, {
      'Content-Type': shell ? 'text/html; charset=utf-8' : asset!.type,
      'Cache-Control': shell
        ? 'no-store'
        : 'public, max-age=31536000, immutable',
      'Content-Length': body.length,
    });
    res.end(req.method === 'HEAD' ? undefined : body);
    return true;
  } catch {
    return false;
  }
}
