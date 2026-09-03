// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const TproxyInboundSettingsSchema = z.object({
  port: z.number().int().min(1).max(65535).default(443),
  hostname: z.string().default(''),
  secret: z.string().default(''),
  siteSource: z.enum(['zip', 'dir', 'upstream']).default('zip'),
  siteDir: z.string().default(''),
  siteUpstream: z.string().default(''),
  carrierMode: z.enum(['https', 'https-lanes', 'websocket', 'websocket-lanes']).default('https'),
  certFile: z.string().default(''),
  keyFile: z.string().default(''),
  routeThroughXray: z.boolean().default(false),
  outboundTag: z.string().default(''),
  routeXrayPort: z.number().int().min(0).max(65535).optional(),
});
export type TproxyInboundSettings = z.infer<typeof TproxyInboundSettingsSchema>;
