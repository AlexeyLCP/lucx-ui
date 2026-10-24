// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { describe, it, expect } from 'vitest';
import { sha256 } from '@noble/hashes/sha2.js';
import { gcm } from '@noble/ciphers/aes.js';
import { utf8ToBytes, bytesToUtf8 } from '@noble/ciphers/utils.js';
import { bytesFromBase64Url } from '@/lib/awg/vpnuri';
import { genCsqttLink, genInboundLinks, genLink } from '@/lib/xray/inbound-link';
import type { Inbound } from '@/schemas/api/inbound';

function csqttInbound(over: Record<string, unknown> = {}): Inbound {
  return {
    protocol: 'csqtt',
    port: 46000,
    listen: '',
    settings: {
      listenAddr: '0.0.0.0:46000',
      password: 'secret',
      subHost: '1.2.3.4',
      vkHashes: 'abcdefghijklmnop1,abcdefghijklmnop2',
      ...((over.settings as object) || {}),
    },
    streamSettings: {},
    sniffing: {},
    ...over,
  } as unknown as Inbound;
}

// Reverses the v2.5.0 csqtt link format (AES-256-GCM, key = SHA256(seed)).
function decryptCsqttLink(link: string): string {
  const raw = bytesFromBase64Url(link.slice('csqtt://'.length));
  const key = sha256(utf8ToBytes('csqtt-chiper-key'));
  const iv = raw.slice(0, 12);
  const plain = gcm(key, iv).decrypt(raw.slice(12));
  return bytesToUtf8(plain);
}

describe('genCsqttLink', () => {
  // Golden vector from an independent node.js crypto implementation of the
  // upstream web_panel.rs encrypt_csqtt_link spec.
  const WANT_LINK =
    'csqtt://4dwo6ogzMZtTXqFQ6_FWD1GrIBdsUuukfrzp6VHITgifc59obEBNTUuxqJ8MWY6Z5ZmY99CuUYbTajo3DqOEW8qPe0pIbpHS0SOl40JUoqF9MVxJrfiqW_1zS5uV49EPE1GnoqtOwqR3F6xzDUQ5qMzlEozkPMsr32g';
  const WANT_PAYLOAD =
    'connect?v=2&host=1.2.3.4&peer=46000&password=secret&hashes=abcdefghijklmnop1+abcdefghijklmnop2';

  it('encrypts the v2 payload like upstream 2.5.0', () => {
    const link = genCsqttLink({ inbound: csqttInbound() });
    expect(link).toBe(WANT_LINK);
    expect(decryptCsqttLink(link)).toBe(WANT_PAYLOAD);
  });

  it('is deterministic (HMAC-derived IV)', () => {
    expect(genCsqttLink({ inbound: csqttInbound() })).toBe(
      genCsqttLink({ inbound: csqttInbound() }),
    );
  });

  it('rejects legacy plaintext links', () => {
    const plain = 'connect?v=2&host=1.2.3.4&peer=46000&password=legacy';
    const legacy =
      'csqtt://' + btoa(plain).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
    expect(() => decryptCsqttLink(legacy)).toThrow();
  });

  it('falls back to address when subHost empty', () => {
    const ib = csqttInbound({
      settings: { subHost: '', password: 'x', listenAddr: '0.0.0.0:46000' },
    });
    const link = genCsqttLink({ inbound: ib, address: '9.9.9.9' });
    expect(decryptCsqttLink(link)).toContain('host=9.9.9.9');
  });

  it('returns empty without password', () => {
    const ib = csqttInbound({ settings: { password: '', subHost: '1.1.1.1' } });
    expect(genCsqttLink({ inbound: ib })).toBe('');
  });

  it('genInboundLinks and genLink dispatch', () => {
    const ib = csqttInbound();
    expect(genInboundLinks({ inbound: ib, remark: 'e2e', fallbackHostname: 'x' })).toContain(
      'csqtt://',
    );
    expect(genLink({ inbound: ib, address: '1.2.3.4', client: {}, remark: 'e2e' })).toBe(WANT_LINK);
  });
});
