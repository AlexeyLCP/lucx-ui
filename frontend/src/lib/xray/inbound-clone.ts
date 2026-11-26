import { RandomUtil } from '@/utils';
import { createDefaultInboundSettings } from '@/lib/xray/inbound-defaults';
import { coerceInboundJsonField, type DBInbound } from '@/models/dbinbound';

type MieruPortBinding = {
  port?: number;
  portRange?: string;
  protocol?: 'TCP' | 'UDP';
};

/*
 * Ports a mieru inbound actually binds, read from settings.portBindings (the
 * row's port mirrors only the primary binding). Used by the clone pre-check
 * and the conflict-aware remap below.
 */
export function mieruBindingPorts(dbInbound: DBInbound): number[] {
  if (dbInbound.protocol !== 'mieru') return [];
  let bindings: MieruPortBinding[] = [];
  try {
    const raw = coerceInboundJsonField(dbInbound.settings);
    if (Array.isArray(raw?.portBindings)) bindings = raw.portBindings as MieruPortBinding[];
  } catch {
    return [];
  }
  const out = new Set<number>();
  for (const b of bindings) {
    if (typeof b.port === 'number') {
      out.add(b.port);
    } else if (b.portRange) {
      const span = /^(\d+)-(\d+)$/.exec(b.portRange.trim());
      if (span) {
        for (let p = parseInt(span[1], 10); p <= parseInt(span[2], 10) && p <= 65535; p++) {
          out.add(p);
        }
      }
    }
  }
  return [...out];
}

function freePort(used: Set<number>, after: number): number | undefined {
  for (let p = after; p <= 65535; p++) {
    if (!used.has(p)) return p;
  }
  return undefined;
}

function remapMieruBinding(b: MieruPortBinding, used: Set<number>): MieruPortBinding {
  if (b.portRange) {
    const span = /^(\d+)-(\d+)$/.exec(b.portRange.trim());
    if (!span) return b;
    const start = parseInt(span[1], 10);
    const size = parseInt(span[2], 10) - start + 1;
    for (let s = start + 1; s + size - 1 <= 65535; s++) {
      let clash = false;
      for (let p = s; p < s + size; p++) {
        if (used.has(p)) {
          clash = true;
          break;
        }
      }
      if (!clash) return { ...b, portRange: `${s}-${s + size - 1}` };
    }
    return b;
  }
  if (typeof b.port !== 'number') return b;
  const next = freePort(used, b.port + 1);
  return next === undefined ? b : { ...b, port: next };
}

/*
 * Shift each binding to the first free port after its original, never onto a
 * source binding and never onto a port bound on the target. A binding with no
 * room is left verbatim so the backend checkMieruPortConflict rejects the
 * whole create with a clear error instead of the clone silently misbehaving.
 */
function remapMieruBindings(
  bindings: MieruPortBinding[],
  used: Set<number> | undefined,
): MieruPortBinding[] {
  if (!used) return bindings;
  const blocked = new Set(used);
  for (const b of bindings) {
    for (const p of mieruBindingPortValues(b)) blocked.add(p);
  }
  // Sequential remap: a chosen port is blocked for the next binding too.
  return bindings.map((b) => {
    const next = remapMieruBinding(b, blocked);
    for (const p of mieruBindingPortValues(next)) blocked.add(p);
    return next;
  });
}

function mieruBindingPortValues(b: MieruPortBinding): number[] {
  if (typeof b.port === 'number') return [b.port];
  if (b.portRange) {
    const span = /^(\d+)-(\d+)$/.exec(b.portRange.trim());
    if (span) {
      const start = parseInt(span[1], 10);
      const end = parseInt(span[2], 10);
      return Array.from({ length: Math.min(end, 65535) - start + 1 }, (_, i) => start + i);
    }
  }
  return [];
}

/*
 * Payload for POST /panel/api/inbounds/add reproducing `dbInbound` as a
 * staged copy: fresh port, empty client list (emails are unique panel-wide
 * and UUIDs must not repeat across nodes), disabled, no tag (the backend
 * regenerates one with the correct per-node prefix), cleared listen (listen
 * addresses are node-local). `nodeId === null` targets the local panel; the
 * field is omitted from the wire payload then, matching the add-form adapter.
 */
export function buildClonePayload(
  dbInbound: DBInbound,
  port: number,
  nodeId: number | null,
  used?: Set<number>,
) {
  let clonedSettings: string;
  try {
    const raw = { ...coerceInboundJsonField(dbInbound.settings) };
    raw.clients = [];
    // Mieru keeps its real listeners inside settings.portBindings; a verbatim
    // copy collides with the source inbound on create (issue #218) because the
    // backend reads bindings, not the row's port. Remap on the operator's
    // explicit Clone click only — never on save/update.
    if (dbInbound.protocol === 'mieru' && Array.isArray(raw.portBindings)) {
      raw.portBindings = remapMieruBindings(raw.portBindings as MieruPortBinding[], used);
    }
    clonedSettings = JSON.stringify(raw);
  } catch {
    const fallback = createDefaultInboundSettings(dbInbound.protocol);
    clonedSettings = fallback ? JSON.stringify(fallback, null, 2) : '{}';
  }
  const streamSettingsString =
    typeof dbInbound.streamSettings === 'string'
      ? dbInbound.streamSettings
      : JSON.stringify(dbInbound.streamSettings ?? {});
  const sniffingString =
    typeof dbInbound.sniffing === 'string'
      ? dbInbound.sniffing
      : JSON.stringify(dbInbound.sniffing ?? {});
  return {
    up: 0,
    down: 0,
    total: 0,
    remark: `${dbInbound.remark} (clone)`,
    enable: false,
    expiryTime: 0,
    listen: '',
    port,
    protocol: dbInbound.protocol,
    settings: clonedSettings,
    streamSettings: streamSettingsString,
    sniffing: sniffingString,
    shareAddrStrategy: dbInbound.shareAddrStrategy,
    shareAddr: dbInbound.shareAddr,
    ...(nodeId != null ? { nodeId } : {}),
  };
}

/*
 * Random clone port in the add-form's range, avoiding ports already bound on
 * the target node (client-side pre-check; the backend's node-scoped conflict
 * check stays the final arbiter). A few random tries cover the common sparse
 * case; a target so dense that those all miss falls back to a deterministic
 * scan so a free port is always found when one exists.
 */
export function pickClonePort(used: Set<number> | undefined): number {
  let port = RandomUtil.randomInteger(10000, 60000);
  if (!used) return port;
  for (let attempts = 0; attempts < 20 && used.has(port); attempts++) {
    port = RandomUtil.randomInteger(10000, 60000);
  }
  if (used.has(port)) {
    for (port = 10000; port <= 60000 && used.has(port); port++) {
      /* dense-range scan */
    }
    if (port > 60000) port = RandomUtil.randomInteger(10000, 60000);
  }
  return port;
}
