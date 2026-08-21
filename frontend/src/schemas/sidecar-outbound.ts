// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const SidecarProtocolSchema = z.enum(['naive', 'mieru', 'trusttunnel']);

export const SidecarOutboundSettingsSchema = z.object({
  socksPort: z.number().int().optional().default(0),
  link: z.string().optional().default(''),
  host: z.string().default(''),
  port: z.number().int().optional().default(0),
  user: z.string().default(''),
  pass: z.string().optional().default(''),
  sni: z.string().optional().default(''),
  alpn: z.string().optional().default(''),
  clientRandomPrefix: z.string().optional().default(''),
  mtu: z.number().int().optional().default(0),
  multiplexing: z.string().optional().default(''),
  handshakeMode: z.string().optional().default(''),
  trafficPatternB64: z.string().optional().default(''),
}).loose();

export const SidecarOutboundSchema = z.object({
  id: z.number().int(),
  protocol: SidecarProtocolSchema,
  tag: z.string(),
  remark: z.string().optional().default(''),
  enable: z.boolean(),
  settings: z.string().optional().default(''),
  created_at: z.number().optional(),
  updated_at: z.number().optional(),
});

export const SidecarOutboundRowSchema = SidecarOutboundSchema.extend({
  status: z.string().optional().default(''),
  binaryExists: z.boolean().optional().default(true),
  binaryMissing: z.boolean().optional().default(false),
});

export const SidecarParseLinkSchema = z.object({
  protocol: SidecarProtocolSchema,
  settings: SidecarOutboundSettingsSchema,
});

export const SidecarTestSchema = z.object({
  ok: z.boolean().optional(),
  latency_ms: z.number().optional(),
  raw: z.string().optional(),
});

export const SidecarBinariesSchema = z.record(
  z.string(),
  z.object({
    exists: z.boolean(),
    path: z.string(),
    name: z.string().optional(),
  }).loose(),
);

export type SidecarProtocol = z.infer<typeof SidecarProtocolSchema>;
export type SidecarOutbound = z.infer<typeof SidecarOutboundSchema>;
export type SidecarOutboundRow = z.infer<typeof SidecarOutboundRowSchema>;
export type SidecarOutboundSettings = z.infer<typeof SidecarOutboundSettingsSchema>;
