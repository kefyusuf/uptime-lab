export type Page =
  { kind: 'create' } | { kind: 'detail'; id: string } | { kind: 'missing' };
export function parsePage(path: string): Page {
  if (path === '/') return { kind: 'create' };
  const match = /^\/monitors\/([A-Za-z0-9-]+)$/.exec(path);
  return match ? { kind: 'detail', id: match[1] } : { kind: 'missing' };
}
export function navigate(path: string): void {
  if (parsePage(path).kind === 'missing')
    throw new Error('Invalid application navigation.');
  window.history.pushState(null, '', path);
  window.dispatchEvent(new PopStateEvent('popstate'));
}
export function subscribeNavigation(callback: () => void): () => void {
  window.addEventListener('popstate', callback);
  return () => window.removeEventListener('popstate', callback);
}
