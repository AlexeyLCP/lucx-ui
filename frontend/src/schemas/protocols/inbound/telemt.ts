import { z } from 'zod';

export const TelemtClientSchema = z.object({
  secret: z.string().default(''),
  email: z.string().min(1),
  enable: z.boolean().default(true),
  totalGB: z.number().int().min(0).default(0),
  expiryTime: z.number().int().default(0),
});

export const TelemtInboundSettingsSchema = z.object({
  tlsDomain: z.string().default('www.cloudflare.com'),
  listen: z.string().default('0.0.0.0'),
  useMiddleProxy: z.boolean().default(true),
  mask: z.boolean().default(true),
  fastMode: z.boolean().default(true),
  clients: z.array(TelemtClientSchema).default([]),
});

export type TelemtInboundSettings = z.infer<typeof TelemtInboundSettingsSchema>;
