// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { PortSchema } from '@/schemas/primitives';

export const TunnelNetworkSchema = z.enum(['tcp', 'udp', 'tcp,udp']);
export type TunnelNetwork = z.infer<typeof TunnelNetworkSchema>;

// Tunnel inbound (Xray's `dokodemo-door`-style transparent forwarder).
// `portMap` is persisted as Record<string, string> on the wire — the panel
// flattens an internal array-of-{name,value} into that map via toV2Headers
// with arr=false.
export const TunnelInboundSettingsSchema = z.object({
  rewriteAddress: z.string().optional(),
  // AntD InputNumber writes null when cleared; accept it and collapse to
  // undefined so the field is omitted from the payload instead of crashing
  // validation with "Invalid input" (issue #5516). The trailing .optional()
  // keeps the key optional in the inferred type (a bare .transform() would
  // make it required).
  rewritePort: PortSchema.nullable()
    .transform((v) => v ?? undefined)
    .optional(),
  portMap: z.record(z.string(), z.string()).default({}),
  allowedNetwork: TunnelNetworkSchema.default('tcp,udp'),
  followRedirect: z.boolean().default(false),
});
export type TunnelInboundSettings = z.infer<typeof TunnelInboundSettingsSchema>;
