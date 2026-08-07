// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { formatInboundLabel } from '@/lib/inbounds/label';
import { normalizeAwgTimer } from '@/lib/awg/timer';
import { awgVersionAtLeast, awgVersionCeiling, preferPublicHost, resolveShareHost } from '@/lib/xray/inbound-link';
import type { AwgVersion } from '@/lib/xray/inbound-link';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

function persistentKeepaliveLine(keepAlive: unknown): string | null {
  const s = normalizeAwgTimer(keepAlive);
  if (s === '0') return null;
  return `PersistentKeepalive = ${s}`;
}

export function isWireguardClient(client: ClientRecord | null | undefined): boolean {
  if (!client) return false;
  return !!(client.privateKey || client.publicKey || client.allowedIPs || client.preSharedKey || client.keepAlive);
}

export function findWireguardInbound(
  client: ClientRecord | null | undefined,
  inboundsById: Record<number, InboundOption>,
): InboundOption | undefined {
  return (client?.inboundIds || [])
    .map((id) => inboundsById[id])
    .find((ib) => ib?.protocol === 'wireguard');
}

export function buildWireguardClientConfig(
  client: ClientRecord,
  inbound: InboundOption | undefined,
  host = window.location.hostname,
  publicHost = '',
): string {
  const endpointHost = resolveShareHost(inbound ?? {}, inbound?.nodeAddress ?? '', preferPublicHost(host, publicHost));
  const address = client.allowedIPs || '10.0.0.2/32';
  const endpoint = `${endpointHost}:${inbound?.port || ''}`;
  const inboundName = inbound ? formatInboundLabel(inbound.tag, inbound.remark) : '';
  const remark = [inboundName, client.email, client.comment].filter(Boolean).join(' - ');
  const lines = [
    '[Interface]',
    `PrivateKey = ${client.privateKey || client.password || ''}`,
    `Address = ${address}`,
    `DNS = ${inbound?.wgDns || '1.1.1.1, 1.0.0.1'}`,
  ];
  if (inbound?.wgMtu && inbound.wgMtu > 0) lines.push(`MTU = ${inbound.wgMtu}`);
  lines.push('');
  if (remark) lines.push(`# ${remark}`);
  lines.push('[Peer]', `PublicKey = ${inbound?.wgPublicKey || ''}`);
  if (client.preSharedKey) lines.push(`PresharedKey = ${client.preSharedKey}`);
  lines.push('AllowedIPs = 0.0.0.0/0, ::/0', `Endpoint = ${endpoint}`);
  const ka = persistentKeepaliveLine(client.keepAlive);
  if (ka) lines.push(ka);
  return lines.join('\n');
}

// LUCX-HOOK: AWG — client .conf builder for AmneziaWG, mirroring buildWireguardClientConfig
// but inserting the Jc/Jmin/Jmax/S1-S4/H1-H4/I1-I5 obfuscation block into [Interface].
// AWG uses the same Curve25519 keypair/PSK/AllowedIPs as WireGuard, so the client
// record shape is identical; only the obfuscation lines (sourced from the inbound
// hints) are AWG-specific.

// isAwgClient reports whether the client carries AWG/WireGuard-style key fields.
// AWG clients use the same fields (privateKey/publicKey/allowedIPs/preSharedKey),
// so the same check applies — the distinction is made by the inbound protocol.
export function isAwgClient(client: ClientRecord | null | undefined): boolean {
  return isWireguardClient(client);
}

// findAwgInbound returns the first AWG inbound attached to the client.
export function findAwgInbound(
  client: ClientRecord | null | undefined,
  inboundsById: Record<number, InboundOption>,
): InboundOption | undefined {
  return (client?.inboundIds || [])
    .map((id) => inboundsById[id])
    .find((ib) => ib?.protocol === 'awg');
}

// filterAwgObfuscation trims the backend's pre-rendered AWG obfuscation block
// (the inbound "ceiling" — every field the inbound carries, including S3/S4,
// I1-I5, and HeaderProtectionKey for v3) down to the field set a given export
// version understands. v1.5 keeps Jc/Jmin/Jmax/S1/S2/H1-H4 only; v2 adds S3/S4
// + I1-I5; v3 adds HeaderProtectionKey. Older awg-quick builds reject unknown
// lines ("Line unrecognized"), so a v1/v2 client must never receive a v3 block.
export function filterAwgObfuscation(block: string, version: AwgVersion): string {
  const drop: string[] = [];
  // S3/S4 and I1-I5 are AWG v2+ only.
  if (!awgVersionAtLeast(version, '2')) {
    drop.push('S3 =', 'S4 =', 'I1 =', 'I2 =', 'I3 =', 'I4 =', 'I5 =');
  }
  // HeaderProtectionKey is AWG3-only.
  if (!awgVersionAtLeast(version, '3')) {
    drop.push('HeaderProtectionKey =');
    // AWG3 device-level timers/padding — v1/v2 kernels reject these lines.
    drop.push('ContentPaddingAddition =');
    drop.push('RekeyAfterTime =');
    drop.push('RekeyTimeout =');
    drop.push('RejectAfterTime =');
    drop.push('KeepaliveTimeout =');
    drop.push('MaxHandshakeAttempts =');
  }
  if (drop.length === 0) return block;
  return block
    .split('\n')
    .filter((line) => !drop.some((prefix) => line.startsWith(prefix)))
    .join('\n')
    .replace(/\n{3,}/g, '\n\n')
    .trim();
}

// buildAwgClientConfig builds a full AmneziaWG client .conf: [Interface] with
// the client's keypair, tunnel address, MTU, DNS, and the AWG obfuscation block
// (Jc/S1-S4/H1-H4/I1-I5), then [Peer] with the server public key, PSK, the
// full-tunnel AllowedIPs, and the endpoint. The obfuscation block is trimmed to
// awgVersionExport when provided (≤ the inbound ceiling); absent = the ceiling.
export function buildAwgClientConfig(
  client: ClientRecord,
  inbound: InboundOption | undefined,
  host = window.location.hostname,
  publicHost = '',
  awgVersionExport?: AwgVersion,
): string {
  const endpointHost = resolveShareHost(inbound ?? {}, inbound?.nodeAddress ?? '', preferPublicHost(host, publicHost));
  const address = client.allowedIPs || '10.200.0.2/32';
  const endpoint = `${endpointHost}:${inbound?.port || ''}`;
  const inboundName = inbound ? formatInboundLabel(inbound.tag, inbound.remark) : '';
  const remark = [inboundName, client.email, client.comment].filter(Boolean).join(' - ');
  const lines = [
    '[Interface]',
    `PrivateKey = ${client.privateKey || client.password || ''}`,
    `Address = ${address}`,
    `DNS = ${inbound?.wgDns || '1.1.1.1, 1.0.0.1'}`,
  ];
  if (inbound?.wgMtu && inbound.wgMtu > 0) lines.push(`MTU = ${inbound.wgMtu}`);
  // AWG obfuscation block (Jc/Jmin/Jmax/S1-S4/H1-H4/I1-I5/HPK) — pre-rendered by
  // the backend (inboundAwgHints) as the inbound's ceiling. Clamp it to the
  // requested export version when the client app predates the ceiling (older
  // awg-quick rejects unknown fields). Defaults to the ceiling (inbound.awgVersion).
  if (inbound?.awgObfuscation) {
    const ceiling = awgVersionCeiling(inbound.awgVersion);
    const target = awgVersionExport && awgVersionAtLeast(ceiling, awgVersionExport) ? awgVersionExport : ceiling;
    const trimmed = filterAwgObfuscation(inbound.awgObfuscation, target).trimEnd();
    if (trimmed) lines.push(trimmed);
  }
  lines.push('');
  if (remark) lines.push(`# ${remark}`);
  lines.push('[Peer]', `PublicKey = ${inbound?.wgPublicKey || ''}`);
  if (client.preSharedKey) lines.push(`PresharedKey = ${client.preSharedKey}`);
  lines.push('AllowedIPs = 0.0.0.0/0, ::/0', `Endpoint = ${endpoint}`);
  const ka = persistentKeepaliveLine(client.keepAlive);
  if (ka) lines.push(ka);
  return lines.join('\n');
}
// END LUCX-HOOK
