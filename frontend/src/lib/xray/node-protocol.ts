import { Protocols } from '@/schemas/primitives/protocol';
import type { NodeRecord } from '@/schemas/node';

const UPSTREAM_NODE_PROTOCOLS = new Set<string>([
  Protocols.VLESS,
  Protocols.VMESS,
  Protocols.TROJAN,
  Protocols.SHADOWSOCKS,
  Protocols.HYSTERIA,
  Protocols.WIREGUARD,
]);

const LUCX_NODE_PROTOCOLS = new Set<string>([
  Protocols.AWG,
  Protocols.NAIVE,
  Protocols.OLCRTC,
  Protocols.QWDTT,
  Protocols.MIERU,
  Protocols.TRUSTTUNNEL,
  Protocols.ANYTLS,
  Protocols.TPROXY,
  Protocols.COVER,
]);

export function isProtocolNodeEligible(protocol: string): boolean {
  return UPSTREAM_NODE_PROTOCOLS.has(protocol) || LUCX_NODE_PROTOCOLS.has(protocol);
}

export function isLucxOnlyProtocol(protocol: string): boolean {
  return LUCX_NODE_PROTOCOLS.has(protocol);
}

function nodeFeatureSet(node: NodeRecord): Set<string> | null {
  const features = node.nodeFeatures;
  if (Array.isArray(features) && features.length > 0) {
    return new Set(features.map((f) => String(f).toLowerCase()));
  }
  if (String(node.nodeType || '').toLowerCase() === 'lucx') {
    return null;
  }
  const ver = String(node.panelVersion || '').toLowerCase();
  if (ver.includes('lucx')) {
    return null;
  }
  return new Set();
}

export function nodeSupportsProtocol(node: NodeRecord, protocol: string): boolean {
  if (!isProtocolNodeEligible(protocol)) {
    return false;
  }
  if (node.transitive || !node.id) {
    return false;
  }
  if (!isLucxOnlyProtocol(protocol)) {
    return true;
  }
  const set = nodeFeatureSet(node);
  if (set === null) {
    return true;
  }
  return set.has(protocol.toLowerCase());
}

export function filterNodesForProtocol(nodes: NodeRecord[], protocol: string): NodeRecord[] {
  if (!isProtocolNodeEligible(protocol)) {
    return [];
  }
  return nodes.filter((n) => n.enable && nodeSupportsProtocol(n, protocol));
}
