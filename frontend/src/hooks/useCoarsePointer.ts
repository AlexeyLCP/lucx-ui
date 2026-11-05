// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useCallback, useSyncExternalStore } from 'react';

const QUERY = '(pointer: coarse)';

/**
 * Whether the primary input is a finger (phones, tablets) rather than a mouse.
 * Width alone misses tablets in landscape, which are wide but have no hover
 * and an on-screen keyboard that resizes the page.
 */
export function useCoarsePointer(): boolean {
  const subscribe = useCallback((onChange: () => void) => {
    const mql = window.matchMedia(QUERY);
    mql.addEventListener('change', onChange);
    return () => mql.removeEventListener('change', onChange);
  }, []);
  return useSyncExternalStore(
    subscribe,
    () => window.matchMedia(QUERY).matches,
    () => false,
  );
}
