// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const MasterdnsInboundSettingsSchema = z.object({
  domains: z.string().default(''),
  secret: z.string().default(''),
  encryption: z.number().int().default(2),
  resolvers: z.string().default(''),
  routeThroughXray: z.boolean().default(false),
  outboundTag: z.string().default(''),
});
export type MasterdnsInboundSettings = z.infer<typeof MasterdnsInboundSettingsSchema>;
