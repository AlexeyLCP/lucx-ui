import { Protocols } from '@/schemas/primitives';

/*
 * Protocols whose inbounds can live on a sub-node (the "Deploy To" set).
 * Everything else (http, mixed, tunnel, tun) is panel-local only. The sidecar
 * protocols run on the node's own panel; the backend refuses a node too old.
 * Shared by the inbound form's Deploy To selector and the clone dialog's
 * target picker so the two surfaces can never drift apart.
 */
export const NODE_ELIGIBLE_PROTOCOLS: Readonly<Record<string, true>> = {
  [Protocols.VLESS]: true,
  [Protocols.VMESS]: true,
  [Protocols.TROJAN]: true,
  [Protocols.SHADOWSOCKS]: true,
  [Protocols.HYSTERIA]: true,
  [Protocols.WIREGUARD]: true,
  [Protocols.MTPROTO]: true,
  [Protocols.AMNEZIAWG]: true,
  [Protocols.TUIC]: true,
  // LUCX-HOOK: LucX sidecars deploy to LucX-capable nodes.
  [Protocols.AWG]: true,
  [Protocols.NAIVE]: true,
  [Protocols.OLCRTC]: true,
  [Protocols.QWDTT]: true,
  [Protocols.MIERU]: true,
  [Protocols.TRUSTTUNNEL]: true,
  [Protocols.ANYTLS]: true,
  [Protocols.TPROXY]: true,
  [Protocols.COVER]: true,
  [Protocols.GATEWAY]: true,
  // END LUCX-HOOK
};
