// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { NaiveClientSchema } from './naive';

export const MieruPortBindingSchema = z.object({
  port: z.number().int().min(1025).max(65535).optional(),
  portRange: z.string().optional(),
  protocol: z.enum(['TCP', 'UDP']).default('TCP'),
});
export type MieruPortBinding = z.infer<typeof MieruPortBindingSchema>;

export const MieruInboundSettingsSchema = z.object({
  portBindings: z.array(MieruPortBindingSchema).default([{ port: 20100, protocol: 'TCP' }]),
  mtu: z.number().int().min(1280).max(1500).default(1400),
  loggingLevel: z.enum(['DEBUG', 'INFO', 'WARN', 'ERROR']).default('INFO'),
  routeThroughXray: z.boolean().default(false),
  outboundTag: z.string().default(''),
  routeXrayPort: z.number().int().min(0).max(65535).optional(),
  clients: z.array(NaiveClientSchema).default([]),
});
export type MieruInboundSettings = z.infer<typeof MieruInboundSettingsSchema>;
