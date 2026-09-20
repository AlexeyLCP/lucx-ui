// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

export function maskingPanelURL(
  host: string,
  port: number,
  tls: boolean,
  basePath: string,
): string {
  const proto = tls ? 'https:' : 'http:';
  const skip = port <= 0 || (tls && port === 443) || (!tls && port === 80);
  const base = basePath.endsWith('/') ? basePath : `${basePath}/`;
  return `${proto}//${host}${skip ? '' : `:${port}`}${base}panel/masking`;
}
