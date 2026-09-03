// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

// NaiveProxy tunnel core config — mirrors tunnel.NaiveConfig on the Go side
// (internal/lucx/tunnel/naive.go). The panel renders it into a Caddyfile for
// the supervised caddy process.
export const NaiveConfigSchema = z.object({
  remark: z.string().default(''),
  enabled: z.boolean().default(false),
  listen: z.string().default(''),
  port: z.number().int().min(1).max(65535).default(443),
  domain: z.string().default(''),
  useAcme: z.boolean().default(false),
  acmeEmail: z.string().default(''),
  certFile: z.string().default(''),
  keyFile: z.string().default(''),
  authUser: z.string().default(''),
  authPass: z.string().default(''),
  enableH3: z.boolean().default(true),
  probeResistance: z.boolean().default(true),
  logLevel: z.enum(['DEBUG', 'INFO', 'WARN', 'ERROR']).default('WARN'),
  extraArgs: z.string().default(''),
  routeThroughXray: z.boolean().default(false),
  routeXrayPort: z.number().int().min(0).max(65535).default(0),
  outboundTag: z.string().default(''),
  useRawConfig: z.boolean().default(false),
  rawConfig: z.string().default(''),
});

export type NaiveConfig = z.infer<typeof NaiveConfigSchema>;

// Three-level health probe: running (process alive) -> listening (TCP port
// answers) -> responding (TLS handshake completes). See tunnel.Status.
export const TunnelProbeSchema = z.object({
  running: z.boolean(),
  listening: z.boolean(),
  responding: z.boolean(),
});

export type TunnelProbe = z.infer<typeof TunnelProbeSchema>;

export const NaiveStatusSchema = z.object({
  core: z.string(),
  displayName: z.string(),
  binaryExists: z.boolean(),
  binaryPath: z.string(),
  clientUrl: z.string(),
  config: NaiveConfigSchema,
  probe: TunnelProbeSchema,
  lastLog: z.string(),
});

export type NaiveStatus = z.infer<typeof NaiveStatusSchema>;

// olcRTC tunnel core config — mirrors tunnel.OlcrtcConfig
// (internal/lucx/tunnel/olcrtc.go).
export const OlcrtcConfigSchema = z.object({
  remark: z.string().default(''),
  enabled: z.boolean().default(false),
  provider: z.enum(['jitsi', 'telemost', 'wbstream']).default('jitsi'),
  roomId: z.string().default(''),
  cryptoKey: z.string().default(''),
  transport: z
    .enum(['datachannel', 'vp8channel', 'seichannel', 'videochannel'])
    .default('datachannel'),
  dns: z.string().default('8.8.8.8:53'),
  vp8Fps: z.number().int().min(1).max(120).default(60),
  vp8Batch: z.number().int().min(1).max(64).default(64),
  seiFps: z.number().int().min(1).max(120).default(30),
  seiBatch: z.number().int().min(1).max(64).default(64),
  seiFrag: z.number().int().min(1).default(900),
  seiAck: z.number().int().min(1).default(2000),
  videoW: z.number().int().min(1).default(1080),
  videoH: z.number().int().min(1).default(1080),
  videoFps: z.number().int().min(1).max(120).default(30),
  videoCodec: z.enum(['qrcode', 'tile']).default('qrcode'),
  debug: z.boolean().default(false),
});

export type OlcrtcConfig = z.infer<typeof OlcrtcConfigSchema>;

export const OlcrtcStatusSchema = z.object({
  core: z.string(),
  displayName: z.string(),
  binaryExists: z.boolean(),
  binaryPath: z.string(),
  clientUri: z.string(),
  config: OlcrtcConfigSchema,
  probe: TunnelProbeSchema,
  lastLog: z.string(),
});

export type OlcrtcStatus = z.infer<typeof OlcrtcStatusSchema>;

// qWDTT tunnel core config — mirrors tunnel.QwdttConfig
// (internal/lucx/tunnel/qwdtt.go).
export const QwdttConfigSchema = z.object({
  remark: z.string().default(''),
  enabled: z.boolean().default(false),
  listenAddr: z.string().default('0.0.0.0:56000'),
  wgPort: z.number().int().min(1).max(65535).default(56001),
  password: z.string().default(''),
  dns: z.string().default('8.8.8.8'),
  configDir: z.string().default(''),
  listenRaw: z.string().default('0.0.0.0:56003'),
  listenDirect: z.string().default(''),
  subHost: z.string().default(''),
  vkHashes: z.string().default(''),
  clientPort: z.number().int().min(1).max(65535).default(9000),
  workers: z.number().int().min(1).max(64).default(16),
});

export type QwdttConfig = z.infer<typeof QwdttConfigSchema>;

export const QwdttStatusSchema = z.object({
  core: z.string(),
  displayName: z.string(),
  binaryExists: z.boolean(),
  binaryPath: z.string(),
  clientUri: z.string(),
  legacyUri: z.string(),
  subJson: z.string(),
  config: QwdttConfigSchema,
  probe: TunnelProbeSchema,
  lastLog: z.string(),
});

export type QwdttStatus = z.infer<typeof QwdttStatusSchema>;

// mieru core status — mirrors service.MieruStatus (inbound-only core: no
// legacy config/lifecycle, binary management + aggregate process state).
export const MieruStatusSchema = z.object({
  core: z.string(),
  displayName: z.string(),
  binaryExists: z.boolean(),
  binaryPath: z.string(),
  probe: TunnelProbeSchema,
  lastLog: z.string(),
});

export type MieruStatus = z.infer<typeof MieruStatusSchema>;

// TrustTunnel core status — mirrors service.TrustTunnelStatus (inbound-only).
export const TrustTunnelStatusSchema = z.object({
  core: z.string(),
  displayName: z.string(),
  binaryExists: z.boolean(),
  binaryPath: z.string(),
  probe: TunnelProbeSchema,
  lastLog: z.string(),
});

export type TrustTunnelStatus = z.infer<typeof TrustTunnelStatusSchema>;

export const AnytlsStatusSchema = z.object({
  core: z.string(),
  displayName: z.string(),
  binaryExists: z.boolean(),
  binaryPath: z.string(),
  probe: TunnelProbeSchema,
  lastLog: z.string(),
});

export type AnytlsStatus = z.infer<typeof AnytlsStatusSchema>;

export const TproxyStatusSchema = AnytlsStatusSchema;
export type TproxyStatus = AnytlsStatus;
