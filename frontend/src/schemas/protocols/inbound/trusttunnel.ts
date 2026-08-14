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
  routeThroughXray: z.boolean().default(false),
  outboundTag: z.string().default(''),
  routeXrayPort: z.number().int().min(0).max(65535).optional(),
  metricsPort: z.number().int().min(0).max(65535).optional(),
  listenPreset: z.enum(['stock', 'fast']).default('fast'),
  clientRandomPrefix: z.string().default(''),
  clients: z.array(NaiveClientSchema).default([]),
});
export type TrustTunnelInboundSettings = z.infer<typeof TrustTunnelInboundSettingsSchema>;
