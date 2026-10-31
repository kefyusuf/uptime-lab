// Only lexical validation belongs here; Go validates the cursor position.
export function validateInventoryQuery(rawQuery: string): boolean {
  if (rawQuery.length > 128 || /[^\x20-\x7e]/.test(rawQuery)) return false;
  if (rawQuery === '') return true;
  const seen = new Set<string>();
  for (const part of rawQuery.split('&')) {
    const match = /^(limit|cursor)=(.+)$/.exec(part);
    if (!match || seen.has(match[1])) return false;
    const [, key, value] = match;
    seen.add(key);
    if (key === 'limit') {
      if (!/^[1-9][0-9]?$/.test(value) || Number(value) > 50) return false;
    } else if (!/^[A-Za-z0-9_-]{88}$/.test(value)) return false;
  }
  return true;
}
