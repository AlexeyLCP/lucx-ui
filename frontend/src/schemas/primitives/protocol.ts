import { z } from 'zod';

export const ProtocolSchema = z.enum([
  'vmess',
  'vless',
  'trojan',
  'shadowsocks',
  'wireguard',
  'hysteria',
  'http',
  'mixed',
  'tunnel',
  'tun',
  'mtproto',
  'amneziawg',
  'awg', // LUCX-HOOK: AmneziaWG sidecar protocol
  'naive', // LUCX-HOOK: NaiveProxy sidecar (inbound model)
  'olcrtc', // LUCX-HOOK: olcRTC sidecar
  'qwdtt', // LUCX-HOOK: qWDTT sidecar
  'mieru', // LUCX-HOOK: mieru sidecar
  'trusttunnel', // LUCX-HOOK: TrustTunnel sidecar
  'anytls', // LUCX-HOOK: AnyTLS sidecar
  'tproxy', // LUCX-HOOK: Telegram WEB proxy
  'cover', // LUCX-HOOK: camouflage site on :80/:443
]);
export type Protocol = z.infer<typeof ProtocolSchema>;

// Const map matching the legacy models/inbound.ts `Protocols` export so
// call sites can swap the import without touching `Protocols.VLESS`-style
// references throughout the codebase. Frozen so downstream code can't
// mutate the dispatch table. TUN is kept here for parity even though the
// Go backend's validator no longer accepts it — existing panel deployments
// may still have TUN inbounds saved that we want to render.
export const Protocols = Object.freeze({
  VMESS: 'vmess',
  VLESS: 'vless',
  TROJAN: 'trojan',
  SHADOWSOCKS: 'shadowsocks',
  WIREGUARD: 'wireguard',
  HYSTERIA: 'hysteria',
  HTTP: 'http',
  MIXED: 'mixed',
  TUNNEL: 'tunnel',
  TUN: 'tun',
  MTPROTO: 'mtproto',
  AMNEZIAWG: 'amneziawg',
  AWG: 'awg', // LUCX-HOOK: AmneziaWG
  NAIVE: 'naive', // LUCX-HOOK: NaiveProxy
  OLCRTC: 'olcrtc', // LUCX-HOOK: olcRTC
  QWDTT: 'qwdtt', // LUCX-HOOK: qWDTT
  MIERU: 'mieru', // LUCX-HOOK: mieru
  TRUSTTUNNEL: 'trusttunnel', // LUCX-HOOK: TrustTunnel
  ANYTLS: 'anytls', // LUCX-HOOK: AnyTLS
  TPROXY: 'tproxy', // LUCX-HOOK: Telegram WEB proxy
  COVER: 'cover', // LUCX-HOOK: camouflage site
});
