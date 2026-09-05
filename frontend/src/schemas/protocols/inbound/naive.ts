import { z } from 'zod';

export const NaiveClientSchema = z.object({
  email: z.string().min(1),
  limitIp: z.number().int().min(0).default(0),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
  enable: z.boolean().default(true),
  tgId: z
    .union([z.number(), z.string()])
    .transform((v) => Number(v) || 0)
    .default(0),
  subId: z.string().default(''),
  comment: z.string().default(''),
  reset: z.number().int().min(0).default(0),
  created_at: z.number().int().optional(),
  updated_at: z.number().int().optional(),
});
export type NaiveClient = z.infer<typeof NaiveClientSchema>;

export const NaiveInboundSettingsSchema = z.object({
  domain: z
    .string()
    .refine((v) => !/[\n\r{}, ]/.test(v))
    .default(''),
  useAcme: z.boolean().default(false),
  acmeEmail: z.string().default(''),
  certFile: z.string().default(''),
  keyFile: z.string().default(''),
  authUser: z.string().default(''),
  authPass: z.string().default(''),
  enableH3: z.boolean().default(true),
  probeResistance: z.boolean().default(true),
  logLevel: z.enum(['DEBUG', 'INFO', 'WARN', 'ERROR']).or(z.literal('')).default('WARN'),
  extraArgs: z.string().default(''),
  routeThroughXray: z.boolean().default(true),
  outboundTag: z.string().optional(),
  routeXrayPort: z.number().int().min(0).max(65535).optional(),
  useRawConfig: z.boolean().default(false),
  rawConfig: z.string().default(''),
  behindCover: z.boolean().default(false),
  authSeed: z.string().optional(),
  clients: z.array(NaiveClientSchema).default([]),
});
export type NaiveInboundSettings = z.infer<typeof NaiveInboundSettingsSchema>;
