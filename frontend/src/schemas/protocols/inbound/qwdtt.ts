import { z } from 'zod';

export const QwdttInboundSettingsSchema = z.object({
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
export type QwdttInboundSettings = z.infer<typeof QwdttInboundSettingsSchema>;
