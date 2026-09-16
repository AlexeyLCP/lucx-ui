// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const GatewayInboundSettingsSchema = z.object({
  publicHost: z.string().default(''),
});
export type GatewayInboundSettings = z.infer<typeof GatewayInboundSettingsSchema>;
