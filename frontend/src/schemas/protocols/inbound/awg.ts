import { z } from 'zod';

// AWG (AmneziaWG) inbound. Served by a kernel-interface sidecar managed by
// internal/awg, not Xray, so it has no stream settings. The settings blob
// stores the server's Curve25519 private key, obfuscation parameters
// (Jc/Jmin/Jmax, S1-S4, H1-H4, CPS I1-I5), and a `clients` array where each
// entry is a peer: `id` = client public key, `password` = PresharedKey.
// The client's own private key is never stored server-side.
export const AwgInboundSettingsSchema = z.object({
  privateKey: z.string().default(''),
  publicKey: z.string().default(''),
  mtu: z.number().int().min(576).max(65535).default(1320),
  dns: z.string().optional(),
  // Obfuscation level: 1 = none, 2 = Jc/Jmin/Jmax + S/H, 3 = full + CPS I1-I5.
  obfLevel: z.number().int().min(1).max(3).default(2),
  mimicryProfile: z.enum(['quic', 'sip', 'dns']).default('quic'),
  region: z.string().default('ru'),
  // AmneziaWG junk/transport obfuscation.
  jc: z.number().int().min(0).default(0),
  jmin: z.number().int().min(0).default(0),
  jmax: z.number().int().min(0).default(0),
  s1: z.number().int().min(0).default(0),
  s2: z.number().int().min(0).default(0),
  s3: z.number().int().min(0).default(0),
  s4: z.number().int().min(0).default(0),
  h1: z.string().default(''),
  h2: z.string().default(''),
  h3: z.string().default(''),
  h4: z.string().default(''),
  // CPS (Connection Proxy Signatures) — only emitted when obfLevel >= 2/3.
  i1: z.string().optional(),
  i2: z.string().optional(),
  i3: z.string().optional(),
  i4: z.string().optional(),
  i5: z.string().optional(),
  // Peers: each client is a WireGuard peer. id = public key, password = PSK.
  clients: z
    .array(
      z.object({
        id: z.string(),
        password: z.string(),
        email: z.string(),
        enable: z.boolean().default(true),
      }),
    )
    .default([]),
  // When set, the AWG kernel interface's decrypted packets flow into a TUN
  // inbound injected into the Xray config so egress obeys routing rules.
  // Mirrors mtproto's routeThroughXray but uses a TUN inbound (raw IP from
  // kernel) instead of a SOCKS loopback (TCP from a userspace sidecar).
  routeThroughXray: z.boolean().optional(),
  outboundTag: z.string().optional(),
});
export type AwgInboundSettings = z.infer<typeof AwgInboundSettingsSchema>;