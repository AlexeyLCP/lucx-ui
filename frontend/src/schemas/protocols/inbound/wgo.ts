// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const WgoPresetMaskings = ['STUN', 'MEDIA', 'AUTO', 'NONE'] as const;

// One obfuscation listener in front of a kernel WireGuard inbound (Phobos
// preset): own public UDP port, XOR key, masking mode. Mirrors
// internal/lucx/tunnel/wgo.go WgoPreset.
export const WgoPresetSchema = z.object({
  id: z.string().default(''),
  name: z.string().min(1),
  extPort: z.number().int().min(1).max(65535),
  key: z.string().min(1).max(255),
  masking: z.enum(WgoPresetMaskings),
  mediaSsrc: z.string().default(''),
  mediaPt: z.number().int().min(0).max(127).default(0),
  mediaClock: z.number().int().min(0).max(90000).default(0),
  maxDummy: z.number().int().min(0).max(1024).default(4),
  idleTimeout: z.number().int().min(5).max(86400).default(300),
  clientWgLocalPort: z.number().int().min(1).max(65535).default(13255),
  isDefault: z.boolean().default(false),
});
export type WgoPreset = z.infer<typeof WgoPresetSchema>;

export const WgoObfuscationSchema = z.object({
  enabled: z.boolean().default(false),
  blockDirectPort: z.boolean().default(false),
  presets: z.array(WgoPresetSchema).default([]),
});
export type WgoObfuscation = z.infer<typeof WgoObfuscationSchema>;
