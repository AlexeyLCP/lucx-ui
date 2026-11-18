// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { NaiveClientSchema } from './naive';

export const TrustTunnelInboundSettingsSchema = z.object({
  hostname: z.string().min(1),
  listen: z.string().default('0.0.0.0:443'),
  ipv6: z.boolean().default(true),
  certFile: z.string().default(''),
  keyFile: z.string().default(''),
  clientDns: z.string().default(''),
  upstreamProtocol: z.enum(['http2', 'http3']).default('http2'),
  routeThroughXray: z.boolean().default(true),
  outboundTag: z.string().default(''),
  routeXrayPort: z.number().int().min(0).max(65535).optional(),
  metricsPort: z.number().int().min(0).max(65535).optional(),
  listenPreset: z.enum(['stock', 'fast']).default('fast'),
  clientRandomPrefix: z.string().default(''),
  // lucx.342: site reverse-proxy target and the auth-failure answer must
  // survive the zod round-trip — absent here they are silently stripped on
  // both editor load and submit (tester report /// fragile, 17.11.2026).
  siteUpstream: z.string().default(''),
  authFailureStatus: z
    .union([z.literal(0), z.literal(403), z.literal(404), z.literal(405), z.literal(407)])
    .default(0),
  // nil on the Go side resolves to true (stored settings predating the flag
  // omit the key and must keep both profiles, see ShareLines).
  shareTcpWithQuic: z.boolean().default(true),
  authSeed: z.string().optional(),
  clients: z.array(NaiveClientSchema).default([]),
});
export type TrustTunnelInboundSettings = z.infer<typeof TrustTunnelInboundSettingsSchema>;
