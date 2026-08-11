import { z } from 'zod';

export const OlcrtcInboundSettingsSchema = z.object({
  provider: z.enum(['jitsi', 'telemost', 'wbstream']).default('jitsi'),
  roomId: z.string().default(''),
  cryptoKey: z.string().default(''),
  transport: z.enum(['datachannel', 'vp8channel']).default('datachannel'),
  dns: z.string().default('8.8.8.8:53'),
  vp8Fps: z.number().int().min(1).max(120).default(60),
  vp8Batch: z.number().int().min(1).max(64).default(64),
  debug: z.boolean().default(false),
});
export type OlcrtcInboundSettings = z.infer<typeof OlcrtcInboundSettingsSchema>;
