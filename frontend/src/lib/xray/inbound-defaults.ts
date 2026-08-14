import { RandomUtil, Wireguard } from '@/utils';

import type { HttpInboundSettings } from '@/schemas/protocols/inbound/http';
import type { HysteriaClient, HysteriaInboundSettings } from '@/schemas/protocols/inbound/hysteria';
import type { MixedInboundSettings } from '@/schemas/protocols/inbound/mixed';
import type { MtprotoClient, MtprotoInboundSettings } from '@/schemas/protocols/inbound/mtproto';
import type { AwgInboundSettings } from '@/schemas/protocols/inbound/awg'; // LUCX-HOOK: AWG
import type { NaiveInboundSettings } from '@/schemas/protocols/inbound/naive'; // LUCX-HOOK: Naive
import type { OlcrtcInboundSettings } from '@/schemas/protocols/inbound/olcrtc';
import type { QwdttInboundSettings } from '@/schemas/protocols/inbound/qwdtt';
import type { MieruInboundSettings } from '@/schemas/protocols/inbound/mieru';
import type { TrustTunnelInboundSettings } from '@/schemas/protocols/inbound/trusttunnel';
import type { ShadowsocksClient, ShadowsocksInboundSettings } from '@/schemas/protocols/inbound/shadowsocks';
import type { TrojanClient, TrojanInboundSettings } from '@/schemas/protocols/inbound/trojan';
import type { TunInboundSettings } from '@/schemas/protocols/inbound/tun';
import type { TunnelInboundSettings } from '@/schemas/protocols/inbound/tunnel';
import type { VlessClient, VlessInboundSettings } from '@/schemas/protocols/inbound/vless';
import type { VmessClient, VmessInboundSettings } from '@/schemas/protocols/inbound/vmess';
import type { WireguardInboundSettings } from '@/schemas/protocols/inbound/wireguard';

// Plain-object factories for protocol clients. Each returns a Zod-parsable
// object matching the wire shape. Random fields (id, password, auth,
// email, subId) call RandomUtil at invocation time — pass them in
// `overrides` for deterministic tests or for forms that pre-seed values.
//
// These replace the legacy `new Inbound.<Settings>.<Client>()` constructors
// and the Inbound.ClientBase machinery. Callers no longer carry the
// XrayCommonClass dependency once the swap lands.

interface ClientBaseSeed {
  email?: string;
  subId?: string;
  limitIp?: number;
  totalGB?: number;
  expiryTime?: number;
  enable?: boolean;
  tgId?: number;
  comment?: string;
  reset?: number;
}

interface ClientBase {
  email: string;
  limitIp: number;
  totalGB: number;
  expiryTime: number;
  enable: boolean;
  tgId: number;
  subId: string;
  comment: string;
  reset: number;
}

function clientBase(seed: ClientBaseSeed = {}): ClientBase {
  return {
    email: seed.email ?? RandomUtil.randomLowerAndNum(10),
    limitIp: seed.limitIp ?? 0,
    totalGB: seed.totalGB ?? 0,
    expiryTime: seed.expiryTime ?? 0,
    enable: seed.enable ?? true,
    tgId: seed.tgId ?? 0,
    subId: seed.subId ?? RandomUtil.randomLowerAndNum(16),
    comment: seed.comment ?? '',
    reset: seed.reset ?? 0,
  };
}

export interface VlessClientSeed extends ClientBaseSeed {
  id?: string;
  flow?: VlessClient['flow'];
}

export function createDefaultVlessClient(seed: VlessClientSeed = {}): VlessClient {
  return {
    id: seed.id ?? RandomUtil.randomUUID(),
    flow: seed.flow ?? '',
    ...clientBase(seed),
  };
}

export interface VmessClientSeed extends ClientBaseSeed {
  id?: string;
  security?: VmessClient['security'];
}

export function createDefaultVmessClient(seed: VmessClientSeed = {}): VmessClient {
  return {
    id: seed.id ?? RandomUtil.randomUUID(),
    security: seed.security ?? 'auto',
    alterId: 0,
    ...clientBase(seed),
  };
}

export interface TrojanClientSeed extends ClientBaseSeed {
  password?: string;
}

export function createDefaultTrojanClient(seed: TrojanClientSeed = {}): TrojanClient {
  return {
    password: seed.password ?? RandomUtil.randomSeq(10),
    ...clientBase(seed),
  };
}

export interface ShadowsocksClientSeed extends ClientBaseSeed {
  method?: string;
  password?: string;
  ssMethod?: string;
}

// Shadowsocks clients ship with an empty `method` on single-user inbounds
// (the parent inbound's method is authoritative); only 2022-blake3 multi-
// user inbounds use the per-client method. Callers pass `ssMethod` to seed
// a method-specific password length when creating a multi-user client.
export function createDefaultShadowsocksClient(seed: ShadowsocksClientSeed = {}): ShadowsocksClient {
  const method = seed.method ?? '';
  const password = seed.password ?? RandomUtil.randomShadowsocksPassword(seed.ssMethod ?? '2022-blake3-aes-256-gcm');
  return {
    method,
    password,
    ...clientBase(seed),
  };
}

export interface HysteriaClientSeed extends ClientBaseSeed {
  auth?: string;
}

export function createDefaultHysteriaClient(seed: HysteriaClientSeed = {}): HysteriaClient {
  return {
    auth: seed.auth ?? RandomUtil.randomSeq(10),
    ...clientBase(seed),
  };
}

// Inbound-settings factories. Each returns a Zod-parsable wire-shape with
// schema defaults already applied — no class instance, no XrayCommonClass.
// Callers (form modals via Step 4, InboundsPage clone via Step 5) call
// these instead of the legacy `Inbound.Settings.getSettings(protocol)`.

export function createDefaultVlessInboundSettings(): VlessInboundSettings {
  return {
    clients: [],
    decryption: 'none',
    encryption: 'none',
    fallbacks: [],
  };
}

export function createDefaultVmessInboundSettings(): VmessInboundSettings {
  return { clients: [] };
}

export function createDefaultTrojanInboundSettings(): TrojanInboundSettings {
  return { clients: [], fallbacks: [] };
}

export interface ShadowsocksInboundSeed {
  method?: ShadowsocksInboundSettings['method'];
  password?: string;
  network?: ShadowsocksInboundSettings['network'];
  ivCheck?: boolean;
}

export function createDefaultShadowsocksInboundSettings(
  seed: ShadowsocksInboundSeed = {},
): ShadowsocksInboundSettings {
  const method = seed.method ?? '2022-blake3-aes-256-gcm';
  return {
    method,
    password: seed.password ?? RandomUtil.randomShadowsocksPassword(method),
    network: seed.network ?? 'tcp,udp',
    clients: [],
    ivCheck: seed.ivCheck ?? false,
  };
}

// Hysteria v1 defaults still emit `version: 2` to match the legacy panel
// constructor — the field discriminates v1 vs v2 inside the same settings
// shape. Callers that explicitly want v1 pass `{ version: 1 }`.
export interface HysteriaInboundSeed {
  version?: 2;
}

export function createDefaultHysteriaInboundSettings(
  seed: HysteriaInboundSeed = {},
): HysteriaInboundSettings {
  return {
    version: seed.version ?? 2,
    clients: [],
  };
}

export function createDefaultHttpInboundSettings(): HttpInboundSettings {
  return {
    accounts: [{ user: RandomUtil.randomLowerAndNum(8), pass: RandomUtil.randomLowerAndNum(12) }],
    allowTransparent: false,
  };
}

export function createDefaultMixedInboundSettings(): MixedInboundSettings {
  return {
    auth: 'password',
    accounts: [{ user: RandomUtil.randomLowerAndNum(8), pass: RandomUtil.randomLowerAndNum(12) }],
    udp: false,
    ip: '127.0.0.1',
  };
}

function domainToHex(domain: string): string {
  return Array.from(new TextEncoder().encode(domain))
    .map((b) => b.toString(16).padStart(2, '0'))
    .join('');
}

// generateMtprotoSecret builds an "ee" FakeTLS secret: the marker, 16 random
// bytes (32 hex chars), then the domain encoded as hex. Mirrors the Go
// model.GenerateFakeTLSSecret; the backend re-derives it on save so this is
// only for immediate display in the form.
export function generateMtprotoSecret(domain: string): string {
  return `ee${RandomUtil.randomSeq(32, { type: 'hex' })}${domainToHex(domain)}`;
}

export function createDefaultMtprotoInboundSettings(): MtprotoInboundSettings {
  return {
    fakeTlsDomain: 'www.cloudflare.com',
    clients: [],
  };
}

// LUCX-HOOK: NaiveProxy inbound defaults (Caddy sidecar, mtproto model).
export function createDefaultNaiveInboundSettings(): NaiveInboundSettings {
  return {
    domain: '',
    useAcme: false,
    acmeEmail: '',
    certFile: '',
    keyFile: '',
    authUser: RandomUtil.randomLowerAndNum(8),
    authPass: RandomUtil.randomSeq(16),
    enableH3: true,
    probeResistance: true,
    logLevel: 'WARN',
    extraArgs: '',
    // Like AWG: traffic enters Xray by default so routing rules apply.
    routeThroughXray: true,
    useRawConfig: false,
    rawConfig: '',
    clients: [],
  };
}

export function createDefaultOlcrtcInboundSettings(): OlcrtcInboundSettings {
  return {
    provider: 'jitsi',
    roomId: '',
    cryptoKey: '',
    transport: 'datachannel',
    dns: '8.8.8.8:53',
    vp8Fps: 60,
    vp8Batch: 64,
    debug: false,
    routeThroughXray: false,
    outboundTag: '',
    routeXrayPort: 0,
  };
}

export function createDefaultQwdttInboundSettings(): QwdttInboundSettings {
  return {
    listenAddr: '0.0.0.0:56000',
    wgPort: 56001,
    password: '',
    dns: '8.8.8.8',
    configDir: '',
    listenRaw: '0.0.0.0:56003',
    listenDirect: '',
    subHost: '',
    vkHashes: '',
    clientPort: 9000,
    workers: 16,
    routeThroughXray: true,
    outboundTag: '',
  };
}

export function createDefaultMieruInboundSettings(): MieruInboundSettings {
  return {
    portBindings: [{ port: 20100, protocol: 'TCP' }],
    mtu: 1400,
    loggingLevel: 'INFO',
    routeThroughXray: false,
    outboundTag: '',
    clients: [],
  };
}

export function createDefaultTrustTunnelInboundSettings(): TrustTunnelInboundSettings {
  return {
    hostname: '',
    listen: '0.0.0.0:443',
    ipv6: true,
    certFile: '',
    keyFile: '',
    clientDns: '',
    upstreamProtocol: 'http2',
    routeThroughXray: false,
    outboundTag: '',
    listenPreset: 'fast',
    clientRandomPrefix: '',
    clients: [],
  };
}

// createDefaultMtprotoClient seeds a new MTProto client with a fresh FakeTLS
// secret fronting the given domain. Mirrors the WireGuard client default: the
// backend re-derives the secret on save, so this is only for immediate display.
export function createDefaultMtprotoClient(domain: string): Partial<MtprotoClient> {
  return {
    secret: generateMtprotoSecret(domain || 'www.cloudflare.com'),
  };
}

// LUCX-HOOK: AWG — generate a server keypair and default obfuscation so the
// form has working values before the backend persists them. The backend's
// internal/awg package regenerates obfuscation when obfLevel/profile change,
// so the values here are immediate-display defaults only.
export function createDefaultAwgInboundSettings(): AwgInboundSettings {
  const kp = Wireguard.generateKeypair();
  const r = (min: number, max: number) => min + Math.floor(Math.random() * (max - min + 1));
  return {
    privateKey: kp.privateKey,
    publicKey: kp.publicKey,
    address: '10.200.0.1/24', // server tunnel address (matches defaultAwgBase in client_awg.go)
    // 1420 = 1500 (typical Ethernet) minus WireGuard/AWG's own overhead — the
    // throughput-optimal value on a normal VPS/hosting network. Drop to 1320
    // (the old default) only behind an extra encapsulation hop that eats more
    // headroom: mobile/CGNAT, PPPoE, or a server that is itself reached over
    // another tunnel. See awgMtuHint in the form for the same guidance.
    mtu: 1420,
    dns: '1.1.1.1, 1.0.0.1',
    obfLevel: 2,
    mimicryProfile: 'quic',
    browserProfile: 'chrome',
    region: 'ru',
    jc: r(3, 10),
    jmin: r(50, 100),
    jmax: r(150, 250),
    s1: r(20, 100),
    s2: r(20, 100),
    s3: r(20, 100),
    s4: r(20, 100),
    h1: `${r(100000, 200000)}-${r(300000, 500000)}`,
    h2: `${r(600000000, 700000000)}-${r(800000000, 900000000)}`,
    h3: `${r(1100000000, 1200000000)}-${r(1300000000, 1400000000)}`,
    h4: `${r(1700000000, 1800000000)}-${r(1900000000, 2000000000)}`,
    // AWG3 header protection key — left empty by default. It is written to the
    // .conf only when awgVersion === '3' (the inbound opts into AWG3); operators
    // fill it via "Regenerate obfuscation" with version '3' selected. S1-S4 are
    // always >= 12 (range starts at 20) so the kernel accepts the key for v3.
    headerProtectionKey: '',
    // AWG3 device-level timers/padding — 0 = kernel uses the built-in WG
    // constant (RekeyAfterTime=120, RekeyTimeout=5, RejectAfterTime=180,
    // KeepaliveTimeout=10, MaxHandshakeAttempts=18, ContentPadding=0). The
    // operator overrides per-inbound in the AWG3 advanced section.
    contentPaddingAddition: '0',
    rekeyAfterTime: '0',
    rekeyTimeout: '0',
    rejectAfterTime: '0',
    keepaliveTimeout: '0',
    maxHandshakeAttempts: '0',
    randomTrailers: false,
    disableCookies: false,
    // AWG protocol version '2' is the safe default — no HeaderProtectionKey,
    // accepted by any kernel. Bump to '3' for AWG3 clients (desktop 5.0.0.5 /
    // Android 3.0.1); note a v3 server will NOT accept v1/v2/plain-WG clients.
    // H1-H4 are "lo-hi" ranges as the v2+ form requires; when the operator
    // picks version '1.5' and clicks "Regenerate obfuscation", the backend
    // (GenerateAWGParams with awgVersion) returns single integers instead —
    // the v1.x awg-quick parser rejects the range form. Defaults are only the
    // initial seed; the generator is the source of truth for the wire format.
    awgVersion: '2',
    routeThroughXray: true,
    outboundTag: '',
    clients: [],
  };
}
// END LUCX-HOOK

export function createDefaultTunnelInboundSettings(): TunnelInboundSettings {
  return {
    portMap: {},
    allowedNetwork: 'tcp,udp',
    followRedirect: false,
  };
}

export function createDefaultTunInboundSettings(): TunInboundSettings {
  return {
    name: 'xray0',
    mtu: 1500,
    gateway: [],
    dns: [],
    userLevel: 0,
    autoSystemRoutingTable: [],
    autoOutboundsInterface: 'auto',
  };
}

export interface WireguardInboundSeed {
  mtu?: number;
  secretKey?: string;
  noKernelTun?: boolean;
}

// WireGuard is multi-client now: a new inbound holds only the server identity
// (secretKey/mtu) and starts with no clients. Clients (peers) are added later
// through the client modal, which generates each one's keypair and a unique
// tunnel address. peers stays empty for backward-compatible parsing.
export function createDefaultWireguardInboundSettings(
  seed: WireguardInboundSeed = {},
): WireguardInboundSettings {
  return {
    mtu: seed.mtu ?? 1420,
    secretKey: seed.secretKey ?? Wireguard.generateKeypair().privateKey,
    peers: [],
    clients: [],
    noKernelTun: seed.noKernelTun ?? false,
  };
}

// Protocol-aware dispatch over every inbound-settings factory. Mirrors
// the legacy `Inbound.Settings.getSettings(protocol)` dispatcher, but
// returns a plain Zod-parsable object instead of a class instance.
// Callers swapping off the class hierarchy use this in place of
// `getSettings(p)` + `.toJson()`.
export type AnyInboundSettings =
  | VlessInboundSettings
  | VmessInboundSettings
  | TrojanInboundSettings
  | ShadowsocksInboundSettings
  | HysteriaInboundSettings
  | HttpInboundSettings
  | MixedInboundSettings
  | TunInboundSettings
  | TunnelInboundSettings
  | WireguardInboundSettings
  | MtprotoInboundSettings
  | AwgInboundSettings // LUCX-HOOK: AWG
  | NaiveInboundSettings
  | OlcrtcInboundSettings
  | QwdttInboundSettings
  | MieruInboundSettings
  | TrustTunnelInboundSettings;

export function createDefaultInboundSettings(protocol: string): AnyInboundSettings | null {
  switch (protocol) {
    case 'vless':       return createDefaultVlessInboundSettings();
    case 'vmess':       return createDefaultVmessInboundSettings();
    case 'trojan':      return createDefaultTrojanInboundSettings();
    case 'shadowsocks': return createDefaultShadowsocksInboundSettings();
    case 'hysteria':    return createDefaultHysteriaInboundSettings();
    case 'http':        return createDefaultHttpInboundSettings();
    case 'mixed':       return createDefaultMixedInboundSettings();
    case 'tunnel':      return createDefaultTunnelInboundSettings();
    case 'tun':         return createDefaultTunInboundSettings();
    case 'wireguard':   return createDefaultWireguardInboundSettings();
    case 'mtproto':     return createDefaultMtprotoInboundSettings();
    case 'awg':         return createDefaultAwgInboundSettings(); // LUCX-HOOK: AWG
    case 'naive':       return createDefaultNaiveInboundSettings(); // LUCX-HOOK: Naive
    case 'olcrtc':      return createDefaultOlcrtcInboundSettings();
    case 'qwdtt':       return createDefaultQwdttInboundSettings();
    case 'mieru':       return createDefaultMieruInboundSettings();
    case 'trusttunnel': return createDefaultTrustTunnelInboundSettings();
    default:            return null;
  }
}
