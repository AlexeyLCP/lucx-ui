// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Client-side builders for the wgobfs engine (Phobos fork of ClusterM
// wg-obfuscator). Formats mirror internal/lucx/tunnel/wgo.go — the golden
// tests in frontend/src/test/wgo-link.test.ts and the Go goldens must stay
// byte-identical.

import type { WgoPreset } from '@/schemas/protocols/inbound/wgo';

export function base64UrlNoPad(s: string): string {
  const bytes = new TextEncoder().encode(s);
  let bin = '';
  bytes.forEach((b) => {
    bin += String.fromCharCode(b);
  });
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

// Client-side [instance] block of one preset: the local obfuscator listens on
// loopback clientWgLocalPort and forwards to host:extPort (Phobos link spec).
export function instanceSection(preset: WgoPreset, host: string): string {
  const lines = [
    '[instance]',
    'source-if = 127.0.0.1',
    `source-lport = ${preset.clientWgLocalPort}`,
    `target = ${host}:${preset.extPort}`,
    `key = ${preset.key}`,
    `masking = ${preset.masking}`,
    'verbose = INFO',
    `idle-timeout = ${preset.idleTimeout}`,
    `max-dummy = ${preset.maxDummy}`,
  ];
  if (preset.masking === 'MEDIA' && preset.mediaSsrc.trim() !== '') {
    lines.push(`media-ssrc = ${preset.mediaSsrc.trim()}`);
  }
  return lines.join('\n') + '\n';
}

// The WireGuard endpoint a preset-bound client dials: the local obfuscator,
// never the server directly.
export function obfuscatedEndpoint(preset: WgoPreset): string {
  return `127.0.0.1:${preset.clientWgLocalPort}`;
}

// Standalone INI for upstream ClusterM clients (Windows CLI, OpenWrt/LuCI,
// MikroTik) — only meaningful for NONE/STUN presets (fork-only modes need
// fork client builds).
export function clustermIni(preset: WgoPreset, host: string): string {
  const lines = [
    '[main]',
    `source-lport = ${preset.clientWgLocalPort}`,
    `target = ${host}:${preset.extPort}`,
    `key = ${preset.key}`,
    `masking = ${preset.masking}`,
    `max-dummy = ${preset.maxDummy}`,
    `idle-timeout = ${preset.idleTimeout}`,
    'verbose = 2',
  ];
  if (preset.masking === 'MEDIA' && preset.mediaSsrc.trim() !== '') {
    lines.push(`media-ssrc = ${preset.mediaSsrc.trim()}`);
  }
  return lines.join('\n') + '\n';
}

// phobos:// share link: base64url (unpadded) of the whole client .conf with
// the URL-encoded client name in the fragment.
export function phobosLink(conf: string, name: string): string {
  return `phobos://${base64UrlNoPad(conf)}#${encodeURIComponent(name)}`;
}

export interface WgosocksLinkPreset {
  key: string;
  masking: string;
  mediaSsrc?: string;
}

// Client payload of one wgosocks client: [instance] (role=client) plus the
// separate [socks5] credentials the client app reads to build
// socks5://login:password@127.0.0.1:1080. Mirrors WgosocksClientConf in Go.
export function wgosocksClientConf(
  host: string,
  port: number,
  login: string,
  password: string,
  preset: WgosocksLinkPreset,
): string {
  const lines = [
    '[instance]',
    'mode = socks5',
    'role = client',
    'source-if = 127.0.0.1',
    'source-lport = 1080',
    `target = ${host}:${port}`,
    `key = ${preset.key}`,
    `masking = ${preset.masking}`,
    'verbose = INFO',
  ];
  if (preset.masking === 'MEDIA' && preset.mediaSsrc && preset.mediaSsrc.trim() !== '') {
    lines.push(`media-ssrc = ${preset.mediaSsrc.trim()}`);
  }
  lines.push('', '[socks5]', `login = ${login}`, `password = ${password}`);
  return lines.join('\n') + '\n';
}

// phobos:// link of one wgosocks client (payload = wgosocksClientConf).
export function wgosocksPhobosLink(
  name: string,
  host: string,
  port: number,
  login: string,
  password: string,
  preset: WgosocksLinkPreset,
): string {
  return phobosLink(wgosocksClientConf(host, port, login, password, preset), name);
}
