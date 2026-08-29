// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const AnytlsInboundSettingsSchema = z.object({
  port: z.number().int().min(1).max(65535).default(8443),
  password: z.string().default(''),
  sni: z.string().default(''),
  certFile: z.string().default(''),
  keyFile: z.string().default(''),
});
export type AnytlsInboundSettings = z.infer<typeof AnytlsInboundSettingsSchema>;
