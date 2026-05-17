// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// AWG config generator — produces valid .conf file text from inbound settings.
// Isolated from leagcy inbound.js to keep protocol-specific logic separate.

/**
 * Generate a complete AmneziaWG client config (.conf) from inbound data.
 * @param {Object} inbound - parsed Inbound model instance
 * @param {string} address - server IP/hostname
 * @param {number} port - server port
 * @param {string} remark - client remark/name
 * @param {Object} client - client object with id/pubkey, password/psk, email
 * @returns {string} valid AWG .conf file content
 */
export function generateAWGConfig(inbound, address, port, remark, client) {
  const s = inbound.settings || {};

  // === Resolve effective client ===
  const clients = Array.isArray(s.clients) ? s.clients : [];
  const effectiveClient = client || clients[0] || {};

  // === Keys: nullish coalescing (0 is valid for numbers, but keys are strings) ===
  const privateKey = effectiveClient.privateKey || effectiveClient.id || s.privateKey || '';
  const publicKey  = effectiveClient.id || s.publicKey || '';
  const presharedKey = effectiveClient.password || s.presharedKey || '';

  // === Obfuscation: use nullish coalescing so 0 doesn't fall through ===
  const mtu   = s.mtu   ?? 1320;
  const jc    = s.jc    ?? 8;
  const jmin  = s.jmin  ?? 50;
  const jmax  = s.jmax  ?? 500;
  const ss1   = s.s1    ?? 50;
  const ss2   = s.s2    ?? 80;
  const ss3   = s.s3    ?? 30;
  const ss4   = s.s4    ?? 15;
  const h1    = s.h1    || '88830977-466888999';
  const h2    = s.h2    || '577571549-1039919960';
  const h3    = s.h3    || '1167874883-1558472606';
  const h4    = s.h4    || '1739740840-2061202155';
  const i1    = s.i1 || effectiveClient.i1 || '';
  const i2    = s.i2 || effectiveClient.i2 || '';
  const i3    = s.i3 || effectiveClient.i3 || '';
  const i4    = s.i4 || effectiveClient.i4 || '';
  const i5    = s.i5 || effectiveClient.i5 || '';

  // === Client address ===
  const clientIndex = effectiveClient.email
    ? clients.findIndex(c => c.email === effectiveClient.email)
    : -1;
  const nextOctet = clientIndex >= 0 ? (2 + clientIndex) : (2 + clients.length);
  const clientAddr = effectiveClient.address
    || `10.${port % 255}.0.${nextOctet >= 2 ? nextOctet : 2}/32`;

  // === Build config ===
  const isDisabled = effectiveClient.enable === false;
  let conf = '';
  if (isDisabled) conf += '# ⚠ DISABLED CLIENT — will not pass traffic\n';
  conf += '[Interface]\n';
  conf += `PrivateKey = ${privateKey}\n`;
  conf += `Address = ${clientAddr}\n`;
  conf += 'DNS = 1.1.1.1, 1.0.0.1\n';
  conf += `MTU = ${mtu}\n`;
  conf += `Jc = ${jc}\n`;
  conf += `Jmin = ${jmin}\n`;
  conf += `Jmax = ${jmax}\n`;
  conf += `S1 = ${ss1}\n`;
  conf += `S2 = ${ss2}\n`;
  conf += `S3 = ${ss3}\n`;
  conf += `S4 = ${ss4}\n`;
  conf += `H1 = ${h1}\n`;
  conf += `H2 = ${h2}\n`;
  conf += `H3 = ${h3}\n`;
  conf += `H4 = ${h4}\n`;

  if (i1) conf += `I1 = <b 0x${i1}>\n`;
  if (i2) conf += `I2 = <b 0x${i2}>\n`;
  if (i3) conf += `I3 = <b 0x${i3}>\n`;
  if (i4) conf += `I4 = <b 0x${i4}>\n`;
  if (i5) conf += `I5 = <b 0x${i5}>\n`;

  conf += '\n[Peer]\n';
  conf += `PublicKey = ${publicKey}\n`;
  conf += `PresharedKey = ${presharedKey}\n`;
  conf += `Endpoint = ${address}:${port}\n`;
  conf += 'AllowedIPs = 0.0.0.0/0, ::/0\n';
  conf += 'PersistentKeepalive = 25\n';

  if (remark) conf = '#' + remark + '\n' + conf;
  return conf;
}

/**
 * Generate a Telemt tg://proxy deep link.
 * @param {string} host - server IP/hostname
 * @param {number} port - server port
 * @param {string} secret - ee-prefixed hex secret
 * @returns {string} tg://proxy deep link
 */
export function generateTelemtLink(host, port, secret) {
  return `tg://proxy?server=${host}&port=${port}&secret=${secret || ''}`;
}
