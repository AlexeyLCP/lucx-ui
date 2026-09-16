import { z } from 'zod';

export const CsqttInboundSettingsSchema = z.object({
  listenAddr: z.string().default('0.0.0.0:46000'),
  password: z.string().default(''),
  subHost: z.string().default(''),
  vkHashes: z.string().default(''),
  routeThroughXray: z.boolean().default(true),
  outboundTag: z.string().default(''),
});
export type CsqttInboundSettings = z.infer<typeof CsqttInboundSettingsSchema>;
