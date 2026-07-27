// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

// AWG (AmneziaWG) inbound. Served by a kernel-interface sidecar managed by
// internal/awg, not Xray, so it has no stream settings. The settings blob
// stores the server's Curve25519 private key, obfuscation parameters
// (Jc/Jmin/Jmax, S1-S4, H1-H4, CPS I1-I5), and a `clients` array where each
// entry is a peer. The client's own Curve25519 keypair, PSK, and tunnel
// address are stored server-side (mirroring WireGuard) so a full client
// .conf and amneziawg:// share-link can be rendered.
export const AwgInboundSettingsSchema = z.object({
  privateKey: z.string().default(''),
  publicKey: z.string().default(''),
  address: z.string().default(''), // server tunnel address, e.g. "10.8.0.1/24"
  mtu: z.number().int().min(576).max(65535).default(1320),
  dns: z.string().optional(),
  // Obfuscation level: 1 = none, 2 = Jc/Jmin/Jmax + S/H, 3 = full + CPS I1-I5.
  obfLevel: z.number().int().min(1).max(3).default(2),
  mimicryProfile: z.enum(['tls', 'quic', 'sip', 'dns']).default('quic'),
  browserProfile: z.enum(['chrome', 'firefox', 'safari']).default('chrome'),
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
  // AWG3 (AmneziaWG 3) — 32-byte ChaCha20 header protection key, base64 (same
  // shape as a WireGuard private key). Forward-compat: the upstream kernel
  // module's feat/awg3 branch parses `HeaderProtectionKey` in setconf; on the
  // current master module the field is left empty so the .conf renderer omits
  // it (an unknown field would make awg setconf reject the config and break
  // reconcile). Populate only after updating to the AWG3 kernel module.
  headerProtectionKey: z.string().default(''),
  // Peers: each client is a WireGuard peer. The client's keypair, PSK, and
  // tunnel address are stored so a full client .conf and share-link can be
  // rendered (mirroring WireGuard). Legacy inbounds stored id/password; the
  // backend maps these to publicKey/preSharedKey.
  clients: z
    .array(
      z.object({
        publicKey: z.string().default(''),
        privateKey: z.string().optional(),
        preSharedKey: z.string().optional(),
        allowedIPs: z.array(z.string()).default([]),
        keepAlive: z.number().int().min(0).optional(),
        email: z.string(),
        enable: z.boolean().default(true),
        // Legacy fields (old inbounds): id = public key, password = PSK.
        id: z.string().optional(),
        password: z.string().optional(),
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
