// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

/** Shared subscription URL builder for every panel surface that shows SUB/JSON/CLASH/AMNEZIA. */

export interface SubSettingsLinks {
  enable?: boolean;
  subURI?: string;
  subJsonEnable?: boolean;
  subJsonURI?: string;
  subClashEnable?: boolean;
  subClashURI?: string;
  subAwgEnable?: boolean;
  subAwgURI?: string;
  publicHost?: string;
}

export interface BuiltSubLinks {
  sub: string;
  json: string;
  clash: string;
  /** AmneziaWG .conf subscription URL */
  amnezia: string;
  /** Same endpoint with ?format=vpn → vpn:// body */
  amneziaVpn: string;
}

export function buildSubLinks(
  settings: SubSettingsLinks | null | undefined,
  subId: string | null | undefined,
): BuiltSubLinks {
  const empty: BuiltSubLinks = { sub: '', json: '', clash: '', amnezia: '', amneziaVpn: '' };
  if (!settings || !subId) return empty;

  const sub =
    settings.enable !== false && settings.subURI
      ? settings.subURI + subId
      : '';
  const json =
    settings.subJsonEnable && settings.subJsonURI
      ? settings.subJsonURI + subId
      : '';
  const clash =
    settings.subClashEnable && settings.subClashURI
      ? settings.subClashURI + subId
      : '';
  const amnezia =
    settings.subAwgEnable && settings.subAwgURI
      ? settings.subAwgURI + subId
      : '';
  const amneziaVpn = amnezia
    ? amnezia.includes('?')
      ? `${amnezia}&format=vpn`
      : `${amnezia}?format=vpn`
    : '';

  return { sub, json, clash, amnezia, amneziaVpn };
}
