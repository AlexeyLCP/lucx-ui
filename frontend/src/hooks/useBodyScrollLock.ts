// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect } from 'react';

/**
 * Freezes the page behind an open sheet. iOS Safari ignores `overflow: hidden`
 * on <body> for touch scrolling, and `overscroll-behavior` only applies when
 * the sheet's own content is long enough to scroll — so with two devices a
 * swipe scrolled the page behind. Pinning <body> with `position: fixed` (and
 * restoring the scroll offset afterwards) blocks that on every browser.
 */
export function useBodyScrollLock(active: boolean) {
  useEffect(() => {
    if (!active) return;
    const { body } = document;
    const y = window.scrollY;
    const prev = {
      position: body.style.position,
      top: body.style.top,
      left: body.style.left,
      right: body.style.right,
      width: body.style.width,
      overflow: body.style.overflow,
    };
    body.style.position = 'fixed';
    body.style.top = `-${y}px`;
    body.style.left = '0';
    body.style.right = '0';
    body.style.width = '100%';
    body.style.overflow = 'hidden';
    return () => {
      Object.assign(body.style, prev);
      window.scrollTo(0, y);
    };
  }, [active]);
}
