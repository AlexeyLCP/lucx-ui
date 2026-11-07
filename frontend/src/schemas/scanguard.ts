// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const ScanguardConfigSchema = z.object({
  enabled: z.boolean(),
  sources: z.array(z.string()),
  whitelist: z.array(z.string()),
});
export type ScanguardConfig = z.infer<typeof ScanguardConfigSchema>;

export const ScanguardFirewallSchema = z.object({
  backend: z.string().optional(),
  ipsetAvailable: z.boolean(),
  rulesV4: z.boolean(),
  rulesV6: z.boolean(),
  v4Entries: z.number(),
  v6Entries: z.number(),
});
export type ScanguardFirewall = z.infer<typeof ScanguardFirewallSchema>;

export const ScanguardStatusSchema = z.object({
  enabled: z.boolean(),
  sources: z.array(z.string()),
  whitelist: z.array(z.string()),
  lastUpdated: z.string(),
  lastError: z.string(),
  firewall: ScanguardFirewallSchema,
});
export type ScanguardStatus = z.infer<typeof ScanguardStatusSchema>;
