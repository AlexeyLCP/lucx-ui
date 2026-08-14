// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { normalizeAwgTimer } from '@/lib/awg/timer';

// AWG (AmneziaWG) outbound — client-mode connection to an upstream VPN server.
// The DB row stores Settings as a JSON string (mirrors Inbound.Settings), so
// AwgOutboundSchema.settings is a string on the wire; the parsed object shape
// is AwgOutboundSettingsSchema, used by parseConf and the form.
export const AwgOutboundSettingsSchema = z.object({
  privateKey: z.string().default(''),
  address: z.string().default(''),
  // 1420 = 1500 (typical Ethernet) minus WireGuard/AWG overhead — optimal
  // when the panel host reaches the upstream directly. Lower it if the host
  // itself sits behind an extra encapsulation hop (mobile/CGNAT, PPPoE).
  mtu: z.number().int().default(1420),
  publicKey: z.string().default(''),
  psk: z.string().default(''),
  endpoint: z.string().default(''),
  keepalive: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  allowedIPs: z.string().default('0.0.0.0/0, ::/0'),
  dns: z.string().default(''),
  jc: z.number().int().default(0),
  jmin: z.number().int().default(0),
  jmax: z.number().int().default(0),
  s1: z.number().int().default(0),
  s2: z.number().int().default(0),
  s3: z.number().int().default(0),
  s4: z.number().int().default(0),
  h1: z.string().default(''),
  h2: z.string().default(''),
  h3: z.string().default(''),
  h4: z.string().default(''),
  i1: z.string().default(''),
  i2: z.string().default(''),
  i3: z.string().default(''),
  i4: z.string().default(''),
  i5: z.string().default(''),
  // AWG3 (AmneziaWG 3) header protection key — 32-byte ChaCha20, base64.
  // Written to the awgo-N .conf only when awgVersion === '3' and non-empty.
  headerProtectionKey: z.string().default(''),
  // AWG3 device-level timers/padding. Kept as strings (single or "lo-hi"
  // range) via normalizeAwgTimer, mirroring the inbound schema — a provider
  // .conf carries ranges ("100-120") that a number field would reject
  // (lucx.74). '0' = kernel default.
  contentPaddingAddition: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  rekeyAfterTime: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  rekeyTimeout: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  rejectAfterTime: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  keepaliveTimeout: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  maxHandshakeAttempts: z.preprocess(normalizeAwgTimer, z.string()).default('0'),
  randomTrailers: z.boolean().default(false),
  disableCookies: z.boolean().default(false),
  // AWG protocol version: '1.5' (legacy), '2' (S3/S4 + I1-I5), '3' (HPK),
  // or '3.1' (RandomTrailers / DisableCookies). Auto-detected by ParseConf.
  awgVersion: z.enum(['1.5', '2', '3', '3.1']).default('2'),
});

export const AwgOutboundSchema = z.object({
  id: z.number().int(),
  tag: z.string(),
  remark: z.string().default(''),
  enable: z.boolean().default(true),
  settings: z.string().default(''),
  created_at: z.number(),
  updated_at: z.number(),
});

// List payload = DB row plus a human-readable Status string derived from the
// live kernel interface by the controller's list handler (see Task 6).
export const AwgOutboundRowSchema = AwgOutboundSchema.extend({
  status: z.string().default(''),
});

export const AwgOutboundStatusSchema = z.object({
  up: z.boolean(),
  handshakeAge: z.string(),
  rx: z.number(),
  tx: z.number(),
  ifname: z.string(),
});

export const AwgOutboundTestSchema = z.object({
  ok: z.boolean(),
  latency_ms: z.number(),
  raw: z.string(),
});

export type AwgOutboundSettings = z.infer<typeof AwgOutboundSettingsSchema>;
export type AwgOutbound = z.infer<typeof AwgOutboundSchema>;
export type AwgOutboundRow = z.infer<typeof AwgOutboundRowSchema>;
export type AwgOutboundStatus = z.infer<typeof AwgOutboundStatusSchema>;
export type AwgOutboundTest = z.infer<typeof AwgOutboundTestSchema>;