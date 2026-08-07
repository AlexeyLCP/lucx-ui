// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';
import { WireguardClientSchema } from './wireguard';

import { normalizeAwgTimer } from '@/lib/awg/timer';

// AWG Client = WireGuard client + legacy поля (id/password)
// Переиспользуем WireguardClientSchema чтобы не дублировать comment/limitIp/etc.
const AwgClientSchema = WireguardClientSchema.extend({
  id: z.string().optional(),
  password: z.string().optional(),
  // AWG3 PersistentKeepalive accepts single or range ("25" / "15-25"); 0 = off.
  keepAlive: z.preprocess(normalizeAwgTimer, z.string()).optional(),
});

// AWG (AmneziaWG) inbound. Served by a kernel-interface sidecar managed by
// internal/awg, not Xray, so it has no stream settings. The settings blob
// stores the server's Curve25519 private key, obfuscation parameters
// (Jc/Jmin/Jmax, S1-S4, H1-H4, CPS I1-I5), and a `clients` array where each
// entry is a peer. The client's own Curve25519 keypair, PSK, and tunnel
// address are stored server-side (mirroring WireGuard) so a full client
// .conf and amneziawg:// share-link can be rendered.
export const AwgInboundSettingsSchema = z.object({
  privateKey: z.string().default(''),
  publicKey: z.string().default(''),
  address: z.string().default(''), // server tunnel address, e.g. "10.200.0.1/24"
  mtu: z.number().int().min(576).max(65535).default(1320),
  dns: z.string().optional(),
  // Obfuscation level: 1 = none, 2 = Jc/Jmin/Jmax + S/H, 3 = full + CPS I1-I5.
  obfLevel: z.number().int().min(1).max(3).default(2),
  mimicryProfile: z.enum(['tls', 'quic', 'sip', 'dns']).default('quic'),
  browserProfile: z.enum(['chrome', 'firefox', 'safari']).default('chrome'),
  region: z.string().default('ru'),
  // AmneziaWG junk/transport obfuscation.
  jc: z.number().int().min(0).default(0),
  jmin: z.number().int().min(0).default(0),
  jmax: z.number().int().min(0).default(0),
  s1: z.number().int().min(0).default(0),
  s2: z.number().int().min(0).default(0),
  s3: z.number().int().min(0).default(0),
  s4: z.number().int().min(0).default(0),
  h1: z.string().default(''),
  h2: z.string().default(''),
  h3: z.string().default(''),
  h4: z.string().default(''),
  // CPS (Connection Proxy Signatures) — only emitted when obfLevel >= 2/3.
  i1: z.string().optional(),
  i2: z.string().optional(),
  i3: z.string().optional(),
  i4: z.string().optional(),
  i5: z.string().optional(),
  // AWG3 (AmneziaWG 3) — 32-byte ChaCha20 header protection key, base64 (same
  // shape as a WireGuard private key). The upstream kernel module
  // (v3.0.20260731) + tools (v3.0.20260730) parse `HeaderProtectionKey` in
  // setconf. Written to the .conf only when awgVersion === '3' (the inbound
  // opts into AWG3); for older versions the field stays empty so v1/v2 kernels
  // keep accepting the config. S1-S4 must be >= 12 for the kernel to accept the
  // key (the generator enforces this; the form warns on manual override).
  headerProtectionKey: z.string().default(''),
  // AWG3 device-level timers/padding (empty/"0" = kernel uses the built-in
  // WireGuard constant). Written to .conf only when non-zero AND awgVersion ===
  // '3'. Kernel defaults: RekeyAfterTime=120s, RekeyTimeout=5s,
  // RejectAfterTime=180s, KeepaliveTimeout=10s, MaxHandshakeAttempts=18,
  // ContentPaddingAddition=0 (deterministic WG padding).
  //
  // Each field is a STRING holding either a single value ("120") or an
  // inclusive range ("100-500"). The kernel's u16_range_t accepts both and
  // randomizes within a range at rekey (same semantics as H1-H4), so the range
  // passes through VERBATIM — normalizeAwgTimer only clamps/orders the input,
  // it never collapses a range to one number.
  contentPaddingAddition: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  rekeyAfterTime: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  rekeyTimeout: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  rejectAfterTime: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  keepaliveTimeout: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  maxHandshakeAttempts: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  // AWG protocol version this inbound targets: '1.5' (legacy: Jc/Jmin/Jmax +
  // S1/S2 + H1-H4), '2' (adds S3/S4 + optional I1-I5, Android 2.0.1), or '3'
  // (adds HeaderProtectionKey, desktop 5.0.0.5 / Android 3.0.1). The server
  // .conf is generated for this version; client configs may be exported at the
  // same version or lower (clients-page export-version selector). Defaults to
  // '2' for compatibility with pre-lucx.50 inbounds.
  awgVersion: z.enum(['1.5', '2', '3']).default('2'),
  // Peers: each client is a WireGuard peer. The client's keypair, PSK, and
  // tunnel address are stored so a full client .conf and share-link can be
  // rendered (mirroring WireGuard). Legacy inbounds stored id/password; the
  // backend maps these to publicKey/preSharedKey.
  clients: z.array(AwgClientSchema).default([]),
  // When set, the AWG kernel interface's decrypted packets flow into a TUN
  // inbound injected into the Xray config so egress obeys routing rules.
  // Mirrors mtproto's routeThroughXray but uses a TUN inbound (raw IP from
  // kernel) instead of a SOCKS loopback (TCP from a userspace sidecar).
  routeThroughXray: z.boolean().optional(),
  outboundTag: z.string().optional(),
});
export type AwgInboundSettings = z.infer<typeof AwgInboundSettingsSchema>;
