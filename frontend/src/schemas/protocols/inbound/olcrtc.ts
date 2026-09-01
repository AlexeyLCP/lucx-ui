import { z } from 'zod';

export const OlcrtcInboundSettingsSchema = z.object({
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
  routeThroughXray: z.boolean().default(false),
  outboundTag: z.string().default(''),
  routeXrayPort: z.number().int().min(0).max(65535).default(0),
});
export type OlcrtcInboundSettings = z.infer<typeof OlcrtcInboundSettingsSchema>;
