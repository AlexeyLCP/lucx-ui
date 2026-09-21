// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const CsqttInboundSettingsSchema = z.object({
  listenAddr: z.string().default('0.0.0.0:46000'),
  password: z.string().default(''),
  deviceId: z.string().default(''),
  subHost: z.string().default(''),
  vkHashes: z.string().default(''),
  routeThroughXray: z.boolean().default(true),
  outboundTag: z.string().default(''),
});
export type CsqttInboundSettings = z.infer<typeof CsqttInboundSettingsSchema>;
