// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const WgosocksMaskings = ['MEDIA', 'TLS', 'AUTO', 'NONE'] as const;

export const WgosocksClientSchema = z.object({
  email: z.string().min(1),
  login: z.string().min(1).max(255),
  password: z.string().min(1).max(255),
  enable: z.boolean().default(true),
});
export type WgosocksClient = z.infer<typeof WgosocksClientSchema>;

export const WgosocksInboundSettingsSchema = z.object({
  key: z.string().min(1).max(255).default(''),
  masking: z.enum(WgosocksMaskings).default('MEDIA'),
  mediaSsrc: z.string().default(''),
  mediaPt: z.number().int().min(0).max(127).default(0),
  mediaClock: z.number().int().min(0).max(90000).default(0),
  maxDummy: z.number().int().min(0).max(1024).default(4),
  idleTimeout: z.number().int().min(5).max(86400).default(300),
  maxClients: z.number().int().min(1).max(1024).default(1024),
  // Clients are edited in the client modal, not this form (anytls pattern):
  // the panel mints login/password on save. Without the field Zod strips
  // them and a no-op save detaches every client.
  clients: z.array(WgosocksClientSchema).default([]),
});
export type WgosocksInboundSettings = z.infer<typeof WgosocksInboundSettingsSchema>;
