// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const QwdttInboundSettingsSchema = z.object({
  listenAddr: z.string().default('0.0.0.0:56000'),
  wgPort: z.number().int().min(1).max(65535).default(56001),
  password: z.string().default(''),
  dns: z.string().default('8.8.8.8'),
  configDir: z.string().default(''),
  listenRaw: z.string().default('0.0.0.0:56003'),
  listenDirect: z.string().default(''),
  subHost: z.string().default(''),
  vkHashes: z.string().default(''),
  clientPort: z.number().int().min(1).max(65535).default(9000),
  workers: z.number().int().min(1).max(64).default(16),
  routeThroughXray: z.boolean().default(true),
  outboundTag: z.string().default(''),
});
export type QwdttInboundSettings = z.infer<typeof QwdttInboundSettingsSchema>;
