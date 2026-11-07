// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

// AIVPN (infosave2007/aivpn) inbound settings. The UDP bind port is the
// inbound row's port; subnet/server-ip/paths default on the backend
// (id-derived subnet, mint-once server key, per-instance data dir).
export const AivpnInboundSettingsSchema = z.object({
  listenHost: z.string().default('0.0.0.0'),
  subnet: z.string().default(''),
  serverIp: z.string().default(''),
  tunName: z.string().default(''),
  mtu: z.number().int().min(0).max(1500).default(0),
  ipv6: z.boolean().default(false),
  ipv6Prefix: z.string().default(''),
  extraArgs: z.string().default(''),
});
export type AivpnInboundSettings = z.infer<typeof AivpnInboundSettingsSchema>;

export const AivpnClientSchema = z.object({
  id: z.string().default(''),
  name: z.string().default(''),
  enabled: z.boolean().default(true),
  key: z.string().default(''),
  bytesIn: z.number().default(0),
  bytesOut: z.number().default(0),
  lastConnected: z.number().default(0),
});
export type AivpnClient = z.infer<typeof AivpnClientSchema>;
