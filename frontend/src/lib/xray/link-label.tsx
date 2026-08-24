import { Tag } from 'antd';
import { Base64 } from '@/utils';

/* Shared parsing + rendering for the "protocol / transport / security"
   labels shown above share links in the QR modal, the client info modal
   and the subscription page. Keeping it in one place means the colour
   scheme and the email/stats stripping stay identical across all three. */

export interface LinkParts {
  protocol: string;
  network: string;
  security: string;
  remark: string;
  port: string;
}

const PROTOCOL_LABELS: Record<string, string> = {
  vless: 'Vless',
  vmess: 'Vmess',
  trojan: 'Trojan',
  ss: 'Shadowsocks',
  shadowsocks: 'Shadowsocks',
  hysteria2: 'Hysteria2',
  hy2: 'Hysteria2',
  hysteria: 'Hysteria',
  wireguard: 'WireGuard',
  wg: 'WireGuard',
  tg: 'MTProto',
  vpn: 'AmneziaWG',
  // LUCX-HOOK: tunnel / AWG share schemes
  naive: 'Naive',
  amneziawg: 'AmneziaWG',
  olcrtc: 'olcRTC',
  qwdtt: 'qWDTT',
  wdtt: 'qWDTT',
  mieru: 'mieru',
  mierus: 'mieru',
  tt: 'TrustTunnel',
};

const PROTOCOL_COLORS: Record<string, string> = {
  Vless: 'geekblue',
  Vmess: 'blue',
  Trojan: 'volcano',
  Shadowsocks: 'purple',
  Hysteria: 'magenta',
  Hysteria2: 'magenta',
  WireGuard: 'cyan',
  MTProto: 'blue',
  AmneziaWG: 'yellow',
  Naive: 'orange',
  olcRTC: 'cyan',
  qWDTT: 'gold',
  mieru: 'geekblue',
  TrustTunnel: 'purple',
};

const SECURITY_COLORS: Record<string, string> = {
  TLS: 'green',
  XTLS: 'green',
  REALITY: 'purple',
  FAKETLS: 'green',
};

const TRANSPORT_COLOR = 'gold';

const TAG_STYLE = { marginInlineEnd: 0, fontWeight: 600, letterSpacing: '0.3px' };

// Reverse of inbound-link.ts's own toBase64Url — base64url (RFC 4648 §5, no
// padding) back to the original unicode text, needed to read the remark/
// endpoint back out of a vpn:// link's opaque payload below.
function fromBase64Url(value: string): string {
  const b64 = value.replace(/-/g, '+').replace(/_/g, '/');
  const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4);
  const binary = atob(padded);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) bytes[i] = binary.charCodeAt(i);
  return new TextDecoder().decode(bytes);
}

/* Pull protocol, transport, security plus the remark and port out of a share
   link. vless/trojan carry network+security as `type`/`security` query params
   and the remark in the URL hash; vmess packs them into the base64 JSON as
   `net`/`tls`/`ps`/`port`. Returns null when the scheme is unknown or the
   payload can't be parsed, so callers fall back to "Link N".

   The remark is shown verbatim: the panel displays the subscription's clean
   (name-only) remarks — the per-client traffic/expiry info is rendered only
   into the body a client app imports, so there is nothing to strip here. */
export function parseLinkParts(link: string): LinkParts | null {
  const trimmed = link.trim();
  // Schemes may include '+' (naive+https://…) — not only [a-z0-9].
  const scheme = /^([a-z0-9+.-]+):\/\//i.exec(trimmed)?.[1]?.toLowerCase() ?? '';
  if (!scheme) return null;

  // LUCX-HOOK: naive+https://user:pass@host:port#email
  if (scheme === 'naive+https' || scheme === 'naive+http') {
    return parseNaiveLink(trimmed, scheme.endsWith('https') ? 'HTTPS' : 'HTTP');
  }

  const baseScheme = scheme.includes('+') ? scheme.split('+')[0] : scheme;
  const protocol =
    PROTOCOL_LABELS[baseScheme] ??
    PROTOCOL_LABELS[scheme] ??
    scheme.charAt(0).toUpperCase() + scheme.slice(1);
  let network = '';
  let security = '';
  let remark = '';
  let port = '';
  if (scheme === 'vmess') {
    try {
      const json = JSON.parse(Base64.decode(trimmed.slice('vmess://'.length).split('#')[0])) as {
        net?: string;
        tls?: string;
        ps?: string;
        port?: string | number;
      };
      network = json.net ?? '';
      security = json.tls ?? '';
      remark = typeof json.ps === 'string' ? json.ps : '';
      port = json.port != null ? String(json.port) : '';
    } catch {
      /* unparseable payload, fall back to protocol only */
    }
  } else if (scheme === 'vpn') {
    /* AmneziaWG's vpn:// links are base64url of a plain .conf text (matching
       the real AmneziaVPN app's own share-link scheme), not a structured URL
       — there's no query string or #hash to read a remark/port from without
       corrupting the payload the app itself needs to decode. The remark and
       endpoint are still in there as plain .conf lines, though, so pull them
       back out directly. */
    try {
      const cfgText = fromBase64Url(trimmed.slice('vpn://'.length));
      remark = /^#\s?(.*)$/m.exec(cfgText)?.[1]?.trim() ?? '';
      port = /^Endpoint\s*=\s*.+:(\d+)\s*$/m.exec(cfgText)?.[1] ?? '';
    } catch {
      /* unparseable payload, fall back to protocol only */
    }
  } else if (scheme === 'olcrtc') {
    const body = trimmed.slice('olcrtc://'.length);
    const hashIdx = body.indexOf('#');
    const main = hashIdx >= 0 ? body.slice(0, hashIdx) : body;
    const at = main.lastIndexOf('@');
    remark = at >= 0 ? main.slice(at + 1) : main;
  } else if (scheme === 'qwdtt' || scheme === 'wdtt') {
    try {
      const url = new URL(trimmed.replace(/^wdtt:/i, 'qwdtt:'));
      remark = url.searchParams.get('name') || url.searchParams.get('peer') || '';
    } catch {
      remark = '';
    }
  } else if (scheme === 'amneziawg') {
    try {
      const url = new URL(trimmed);
      port = url.port || '';
      const hash = url.hash.replace(/^#/, '');
      try {
        remark = decodeURIComponent(hash);
      } catch {
        remark = hash;
      }
    } catch {
      /* fall back */
    }
  } else {
    try {
      const url = new URL(trimmed);
      network = url.searchParams.get('type') ?? '';
      security = url.searchParams.get('security') ?? '';
      /* tg://proxy links (mtproto) carry the port in a `port` query param, not
         the URL authority, so fall back to it when there is no authority port. */
      port = url.port || (url.searchParams.get('port') ?? '');
      const hash = url.hash.replace(/^#/, '');
      try {
        remark = decodeURIComponent(hash);
      } catch {
        remark = hash;
      }
    } catch {
      /* not URL-shaped, fall back to protocol only */
    }
    if (scheme === 'tg') security = 'FakeTLS';
  }
  if (security === 'none') security = '';
  return {
    protocol,
    network: network.toUpperCase(),
    security: security.toUpperCase(),
    remark: remark.trim(),
    port,
  };
}

/** Parse naive+https://user:pass@host:port#remark into LinkParts. */
function parseNaiveLink(link: string, security: string): LinkParts {
  let remark = '';
  let port = '';
  // Strip naive+ so URL() can parse https://…
  const rest = link.replace(/^naive\+/i, '');
  try {
    const url = new URL(rest);
    port = url.port || (security === 'HTTPS' ? '443' : '80');
    if (port === '443' || port === '80') port = '';
    const hash = url.hash.replace(/^#/, '');
    try {
      remark = decodeURIComponent(hash);
    } catch {
      remark = hash;
    }
  } catch {
    const hashIdx = link.indexOf('#');
    if (hashIdx >= 0) {
      try {
        remark = decodeURIComponent(link.slice(hashIdx + 1));
      } catch {
        remark = link.slice(hashIdx + 1);
      }
    }
  }
  return {
    protocol: 'Naive',
    network: '',
    security,
    remark: remark.trim(),
    port,
  };
}

/* The inbound remark and port joined as they appear after the tags, e.g.
   "22:10452". Either piece may be empty. */
export function linkMetaText(parts: LinkParts): string {
  return [parts.remark, parts.port].filter(Boolean).join(':');
}

export function LinkTags({ parts }: { parts: LinkParts }) {
  return (
    <span style={{ display: 'inline-flex', alignItems: 'center', gap: 4, flexShrink: 0 }}>
      <Tag color={PROTOCOL_COLORS[parts.protocol]} style={TAG_STYLE}>
        {parts.protocol}
      </Tag>
      {parts.network && (
        <Tag color={TRANSPORT_COLOR} style={TAG_STYLE}>
          {parts.network}
        </Tag>
      )}
      {parts.security && (
        <Tag color={SECURITY_COLORS[parts.security]} style={TAG_STYLE}>
          {parts.security}
        </Tag>
      )}
    </span>
  );
}
