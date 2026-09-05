const FLAG = 'xui-stale-chunk';

export function isStaleChunkError(err: unknown): boolean {
  const msg = err instanceof Error ? err.message : String(err ?? '');
  return /Failed to fetch dynamically imported module|error loading dynamically imported module|Importing a module script failed/i.test(
    msg,
  );
}

export function reloadOnceOnStaleChunk(err: unknown): boolean {
  if (!isStaleChunkError(err) || sessionStorage.getItem(FLAG)) return false;
  sessionStorage.setItem(FLAG, '1');
  window.location.reload();
  return true;
}
