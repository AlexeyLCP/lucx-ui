// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { NaiveClientSchema } from './naive';

export const MieruPortBindingSchema = z.object({
  port: z.number().int().min(1025).max(65535).optional(),
  portRange: z.string().optional(),
  protocol: z.enum(['TCP', 'UDP']).default('TCP'),
});
export type MieruPortBinding = z.infer<typeof MieruPortBindingSchema>;

export const MIERU_MULTIPLEXING_LEVELS = [
  'MULTIPLEXING_OFF',
  'MULTIPLEXING_LOW',
  'MULTIPLEXING_MIDDLE',
  'MULTIPLEXING_HIGH',
] as const;
export const MieruMultiplexingSchema = z.enum(MIERU_MULTIPLEXING_LEVELS);
export type MieruMultiplexing = z.infer<typeof MieruMultiplexingSchema>;

export const MIERU_HANDSHAKE_MODES = ['HANDSHAKE_STANDARD', 'HANDSHAKE_NO_WAIT'] as const;
export const MieruHandshakeModeSchema = z.enum(MIERU_HANDSHAKE_MODES);
export type MieruHandshakeMode = z.infer<typeof MieruHandshakeModeSchema>;

export const MIERU_NONCE_TYPES = [
  'NONCE_TYPE_RANDOM',
  'NONCE_TYPE_PRINTABLE',
  'NONCE_TYPE_PRINTABLE_SUBSET',
  'NONCE_TYPE_FIXED',
] as const;
export const MieruNonceTypeSchema = z.enum(MIERU_NONCE_TYPES);

export const MIERU_LOW_ENTROPY_MODES = [
  'LOW_ENTROPY_MODE_OFF',
  'LOW_ENTROPY_MODE_32',
  'LOW_ENTROPY_MODE_40',
  'LOW_ENTROPY_MODE_48',
  'LOW_ENTROPY_MODE_56',
] as const;
export const MieruLowEntropyModeSchema = z.enum(MIERU_LOW_ENTROPY_MODES);

export const MIERU_MASK_ROTATIONS = [
  'LOW_ENTROPY_MASK_NO_ROTATION',
  ...Array.from({ length: 15 }, (_, i) => `LOW_ENTROPY_MASK_ROTATE_RIGHT_${i + 1}`),
  ...Array.from({ length: 15 }, (_, i) => `LOW_ENTROPY_MASK_ROTATE_LEFT_${i + 1}`),
] as unknown as readonly [string, ...string[]];
export const MieruLowEntropyMaskRotationSchema = z.enum(MIERU_MASK_ROTATIONS);

export const MieruTcpFragmentSchema = z.object({
  enable: z.boolean().optional(),
  maxSleepMs: z.number().int().min(0).max(100).optional(),
});

export const MieruNoncePatternSchema = z.object({
  type: MieruNonceTypeSchema.optional(),
  applyToAllUDPPacket: z.boolean().optional(),
  minLen: z.number().int().min(0).max(12).optional(),
  maxLen: z.number().int().min(0).max(12).optional(),
  customHexStrings: z
    .array(z.string().regex(/^([0-9a-fA-F]{2}){1,12}$/, 'pages.inbounds.form.mieruTpNonceHexInvalid'))
    .optional(),
});

export const MieruPaddingPatternSchema = z.object({
  maxMiddlePaddingLen: z.number().int().min(0).max(255).optional(),
  maxEndPaddingLen: z.number().int().min(0).max(255).optional(),
});

export const MieruLowEntropyPatternSchema = z.object({
  mode: MieruLowEntropyModeSchema.optional(),
  maskRotation: MieruLowEntropyMaskRotationSchema.optional(),
});

export const MieruTrafficPatternSchema = z.object({
  seed: z.number().int().min(0).max(2147483647).optional(),
  unlockAll: z.boolean().optional(),
  tcpFragment: MieruTcpFragmentSchema.optional(),
  nonce: MieruNoncePatternSchema.optional(),
  padding: MieruPaddingPatternSchema.optional(),
  lowEntropy: MieruLowEntropyPatternSchema.optional(),
});
export type MieruTrafficPattern = z.infer<typeof MieruTrafficPatternSchema>;

export const MieruInboundSettingsSchema = z.object({
  portBindings: z.array(MieruPortBindingSchema).default([{ port: 20100, protocol: 'TCP' }]),
  mtu: z.number().int().min(1280).max(1500).default(1400),
  loggingLevel: z.enum(['DEBUG', 'INFO', 'WARN', 'ERROR']).default('INFO'),
  routeThroughXray: z.boolean().default(false),
  outboundTag: z.string().default(''),
  routeXrayPort: z.number().int().min(0).max(65535).optional(),
  multiplexing: MieruMultiplexingSchema.optional(),
  handshakeMode: MieruHandshakeModeSchema.optional(),
  trafficPattern: MieruTrafficPatternSchema.optional(),
  clients: z.array(NaiveClientSchema).default([]),
});
export type MieruInboundSettings = z.infer<typeof MieruInboundSettingsSchema>;
