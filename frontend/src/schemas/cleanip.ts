// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const CleanIpParamsSchema = z.object({
  port: z.number().int().min(1).max(65535),
  count: z.number().int().min(1).max(100),
  sample: z.number().int().min(8).max(4096),
  timeoutMs: z.number().int().min(100).max(10000),
  tlsCheck: z.boolean(),
  sni: z.string(),
});
export type CleanIpParams = z.infer<typeof CleanIpParamsSchema>;

export const CleanIpResultSchema = z.object({
  address: z.string(),
  port: z.number(),
  latencyMs: z.number(),
  tlsOk: z.boolean(),
});
export type CleanIpResult = z.infer<typeof CleanIpResultSchema>;

export const CleanIpResultsSchema = z.array(CleanIpResultSchema);
