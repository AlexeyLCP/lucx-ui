import { z } from 'zod';

import { HttpInboundSettingsSchema } from './http';
import { HysteriaInboundSettingsSchema } from './hysteria';
import { MixedInboundSettingsSchema } from './mixed';
import { MtprotoInboundSettingsSchema } from './mtproto';
import { AwgInboundSettingsSchema } from './awg'; // LUCX-HOOK: AWG protocol
import { NaiveInboundSettingsSchema } from './naive'; // LUCX-HOOK: NaiveProxy
import { OlcrtcInboundSettingsSchema } from './olcrtc'; // LUCX-HOOK: olcRTC
import { QwdttInboundSettingsSchema } from './qwdtt'; // LUCX-HOOK: qWDTT
import { MieruInboundSettingsSchema } from './mieru'; // LUCX-HOOK: mieru
import { TrustTunnelInboundSettingsSchema } from './trusttunnel'; // LUCX-HOOK: TrustTunnel
import { ShadowsocksInboundSettingsSchema } from './shadowsocks';
import { TrojanInboundSettingsSchema } from './trojan';
import { TunInboundSettingsSchema } from './tun';
import { TunnelInboundSettingsSchema } from './tunnel';
import { VlessInboundSettingsSchema } from './vless';
import { VmessInboundSettingsSchema } from './vmess';
import { WireguardInboundSettingsSchema } from './wireguard';

export * from './http';
export * from './hysteria';
export * from './mixed';
export * from './mtproto';
export * from './awg'; // LUCX-HOOK: AWG protocol
export * from './naive'; // LUCX-HOOK: NaiveProxy
export * from './olcrtc'; // LUCX-HOOK: olcRTC
export * from './qwdtt'; // LUCX-HOOK: qWDTT
export * from './mieru'; // LUCX-HOOK: mieru
export * from './trusttunnel'; // LUCX-HOOK: TrustTunnel
export * from './shadowsocks';
export * from './trojan';
export * from './tun';
export * from './tunnel';
export * from './vless';
export * from './vmess';
export * from './wireguard';

// Tagged-wrapper discriminated union. The discriminator (`protocol`) lives on
// the wrapper, not inside `settings`, mirroring the wire format Xray emits:
//   { protocol: 'vless', settings: { clients: [...], ... }, ... }
// Consumers narrow on `.protocol` and TypeScript narrows `.settings` to the
// matching leaf type.
export const InboundSettingsSchema = z.discriminatedUnion('protocol', [
  z.object({ protocol: z.literal('vmess'),       settings: VmessInboundSettingsSchema }),
  z.object({ protocol: z.literal('vless'),       settings: VlessInboundSettingsSchema }),
  z.object({ protocol: z.literal('trojan'),      settings: TrojanInboundSettingsSchema }),
  z.object({ protocol: z.literal('shadowsocks'), settings: ShadowsocksInboundSettingsSchema }),
  z.object({ protocol: z.literal('wireguard'),   settings: WireguardInboundSettingsSchema }),
  z.object({ protocol: z.literal('hysteria'),    settings: HysteriaInboundSettingsSchema }),
  z.object({ protocol: z.literal('http'),        settings: HttpInboundSettingsSchema }),
  z.object({ protocol: z.literal('mixed'),       settings: MixedInboundSettingsSchema }),
  z.object({ protocol: z.literal('tunnel'),      settings: TunnelInboundSettingsSchema }),
  z.object({ protocol: z.literal('tun'),         settings: TunInboundSettingsSchema }),
  z.object({ protocol: z.literal('mtproto'),     settings: MtprotoInboundSettingsSchema }),
  z.object({ protocol: z.literal('awg'),         settings: AwgInboundSettingsSchema }), // LUCX-HOOK: AWG
  z.object({ protocol: z.literal('naive'),       settings: NaiveInboundSettingsSchema }), // LUCX-HOOK: Naive
  z.object({ protocol: z.literal('olcrtc'),      settings: OlcrtcInboundSettingsSchema }), // LUCX-HOOK: olcRTC
  z.object({ protocol: z.literal('qwdtt'),       settings: QwdttInboundSettingsSchema }), // LUCX-HOOK: qWDTT
  z.object({ protocol: z.literal('mieru'),       settings: MieruInboundSettingsSchema }), // LUCX-HOOK: mieru
  z.object({ protocol: z.literal('trusttunnel'), settings: TrustTunnelInboundSettingsSchema }), // LUCX-HOOK: TrustTunnel
]);
export type InboundSettings = z.infer<typeof InboundSettingsSchema>;
