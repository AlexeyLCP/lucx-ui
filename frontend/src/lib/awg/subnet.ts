// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0
// LUCX-UI: AWG subnet helpers — pure functions to mask and compare IPv4 CIDR
// tunnel addresses so the inbound form can warn when two AWG inbounds share an
// overlapping client subnet (the kernel would install two connected routes for
// the same prefix and the reverse path to one inbound's clients would egress
// via the other — see AGENTS.md Pattern 1e).

function parseIPv4(s: string): number | null {
  const parts = s.split('.');
  if (parts.length !== 4) return null;
  let result = 0;
  for (const p of parts) {
    const octet = Number(p);
    if (!Number.isInteger(octet) || octet < 0 || octet > 255) return null;
    result = (result << 8) | octet;
  }
  return result >>> 0;
}

function toDotted(ip: number): string {
  return [(ip >>> 24) & 0xff, (ip >>> 16) & 0xff, (ip >>> 8) & 0xff, ip & 0xff].join('.');
}

function mask(ip: number, prefix: number): number {
  if (prefix <= 0) return 0;
  if (prefix >= 32) return ip >>> 0;
  const m = (0xffffffff << (32 - prefix)) >>> 0;
  return (ip & m) >>> 0;
}

interface ParsedSubnet {
  ip: number;
  prefix: number;
}

function parseSubnet(addr: string): ParsedSubnet | null {
  if (typeof addr !== 'string') return null;
  const m = addr.trim().match(/^(\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3})\/(\d{1,2})$/);
  if (!m) return null;
  const ip = parseIPv4(m[1]);
  const prefix = Number(m[2]);
  if (ip === null || !Number.isInteger(prefix) || prefix < 0 || prefix > 32) return null;
  return { ip, prefix };
}

// maskSubnet reduces a tunnel address like "10.8.0.1/24" to its network prefix
// "10.8.0.0/24". Returns null for invalid input so callers can treat parse
// failures as "no conflict" rather than throwing.
export function maskSubnet(addr: string): string | null {
  const parsed = parseSubnet(addr);
  if (!parsed) return null;
  return `${toDotted(mask(parsed.ip, parsed.prefix))}/${parsed.prefix}`;
}

// subnetsOverlap reports whether two IPv4 CIDR ranges share any address. Two
// prefixes overlap when their network addresses coincide under the longer of
// the two masks (equivalently, under the shorter mask — both must match). Any
// unparseable operand yields false (no conflict), so a malformed Address field
// never blocks the form.
export function subnetsOverlap(a: string, b: string): boolean {
  const na = parseSubnet(a);
  const nb = parseSubnet(b);
  if (!na || !nb) return false;
  const minPrefix = Math.min(na.prefix, nb.prefix);
  return mask(na.ip, minPrefix) === mask(nb.ip, minPrefix);
}
