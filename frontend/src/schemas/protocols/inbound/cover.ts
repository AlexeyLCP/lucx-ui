// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const CoverRouteSchema = z.object({
  path: z.string().default(''),
  dest: z.string().default(''),
});

export const CoverInboundSettingsSchema = z.object({
  hostname: z.string().default(''),
  siteSource: z.enum(['zip', 'dir', 'upstream']).default('zip'),
  siteDir: z.string().default(''),
  siteUpstream: z.string().default(''),
  certFile: z.string().default(''),
  keyFile: z.string().default(''),
  routes: z.array(CoverRouteSchema).default([]),
});
export type CoverInboundSettings = z.infer<typeof CoverInboundSettingsSchema>;
