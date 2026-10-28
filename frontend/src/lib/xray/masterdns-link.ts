// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// MasterDNSVPN share wrapper: the upstream MasterDnsVPN client imports a
// client_config.toml, so the panel carries that TOML (base64url) in a
// masterdns:// URI instead of a client-app scheme. It shows up in the
// subscription and in the QR so the operator can hand the config over; the
// client app itself never parses the scheme (like the AWG .conf download).

const PREFIX = 'masterdns://v1/';

// Mirrors the Go MasterdnsConfig.RenderClientTOML — both sides must stay in
// agreement for the client package.
function renderClientTOML(settings: {
  domains?: string;
  secret?: string;
  encryption?: number;
  resolvers?: string;
}): string {
  const splitList = (raw: string) =>
    (raw || '')
      .split(/[,;\s]+/)
      .map((s) => s.trim().toLowerCase())
      .filter(Boolean);
  const domains = splitList(settings.domains || '');
  const lines: string[] = [];
  lines.push('# MasterDnsVPN client config — rendered by LucX-UI');
  lines.push(`DOMAINS = [${domains.map((d) => JSON.stringify(d)).join(', ')}]`);
  lines.push(`DATA_ENCRYPTION_METHOD = ${settings.encryption ?? 2}`);
  lines.push(`ENCRYPTION_KEY = ${JSON.stringify((settings.secret || '').trim())}`);
  lines.push(`PROTOCOL_TYPE = "SOCKS5"`);
  lines.push(`LISTEN_IP = "127.0.0.1"`);
  lines.push(`LISTEN_PORT = 18000`);
  lines.push(`RESOLVERS_FILE_PATH = "client_resolvers.txt"`);
  return lines.join('\n');
}

export function resolversFile(resolvers?: string): string {
  const list = (resolvers || '')
    .split(/[,;\s]+/)
    .map((s) => s.trim())
    .filter(Boolean);
  if (list.length > 0) return list.join('\n') + '\n';
  return '8.8.8.8:53\n1.1.1.1:53\n9.9.9.9:53\n';
}

export function genMasterdnsLink(input: {
  inbound: {
    settings: { domains?: string; secret?: string; encryption?: number; resolvers?: string };
  };
}): string {
  const s = input.inbound.settings || {};
  const domains = (s.domains || '').trim();
  const secret = (s.secret || '').trim();
  if (!domains || secret.length < 32) return '';
  const toml = renderClientTOML({
    domains,
    secret,
    encryption: s.encryption ?? 2,
    resolvers: (s.resolvers || '').trim(),
  });
  const bytes = new TextEncoder().encode(toml);
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  const b64 = btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
  return PREFIX + b64;
}

export { renderClientTOML as renderMasterdnsClientTOML };
