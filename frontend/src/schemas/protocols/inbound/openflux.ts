// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const OpenfluxInboundSettingsSchema = z.object({
  secret: z.string().default(''),
  shareHost: z.string().default(''),
  yandexUrl: z.string().default(''),
  mailruUrl: z.string().default(''),
  cupsUrl: z.string().default(''),
  // LUCX-HOOK: shareable carriers since core v0.13.0 (#148); absent on
  // legacy inbound rows, defaults keep them empty.
  vyandexUrl: z.string().default(''),
  boardsUrl: z.string().default(''),
  telemostUrl: z.string().default(''),
  avitoUrl: z.string().default(''),
});
export type OpenfluxInboundSettings = z.infer<typeof OpenfluxInboundSettingsSchema>;
