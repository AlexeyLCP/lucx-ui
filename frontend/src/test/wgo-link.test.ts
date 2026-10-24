// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { describe, expect, it } from 'vitest';

import {
  base64UrlNoPad,
  clustermIni,
  instanceSection,
  obfuscatedEndpoint,
  phobosLink,
  wgosocksClientConf,
  wgosocksPhobosLink,
} from '@/lib/wgo/link';
import type { WgoPreset } from '@/schemas/protocols/inbound/wgo';

function fixturePreset(): WgoPreset {
  return {
    id: 'p1',
    name: 'Phobos RU',
    extPort: 51822,
    key: 'k1',
    masking: 'STUN',
    mediaSsrc: '',
    mediaPt: 0,
    mediaClock: 0,
    maxDummy: 4,
    idleTimeout: 300,
    clientWgLocalPort: 13255,
    isDefault: true,
  };
}

describe('wgo link builders', () => {
  it('instanceSection golden matches the Go renderer', () => {
    expect(instanceSection(fixturePreset(), 'vpn.example.com')).toBe(
      [
        '[instance]',
        'source-if = 127.0.0.1',
        'source-lport = 13255',
        'target = vpn.example.com:51822',
        'key = k1',
        'masking = STUN',
        'verbose = INFO',
        'idle-timeout = 300',
        'max-dummy = 4',
        '',
      ].join('\n'),
    );
  });

  it('MEDIA preset carries media-ssrc only when pinned', () => {
    const media = { ...fixturePreset(), masking: 'MEDIA', mediaSsrc: '305419896' } as WgoPreset;
    expect(instanceSection(media, '1.2.3.4')).toContain('media-ssrc = 305419896');
    const mediaNoSsrc = { ...media, mediaSsrc: '' } as WgoPreset;
    expect(instanceSection(mediaNoSsrc, '1.2.3.4')).not.toContain('media-ssrc');
  });

  it('obfuscatedEndpoint is loopback', () => {
    expect(obfuscatedEndpoint(fixturePreset())).toBe('127.0.0.1:13255');
  });

  it('clustermIni renders a standalone main section', () => {
    const ini = clustermIni(fixturePreset(), 'vpn.example.com');
    expect(ini).toContain('[main]');
    expect(ini).toContain('source-lport = 13255');
    expect(ini).toContain('target = vpn.example.com:51822');
    expect(ini).toContain('masking = STUN');
  });

  it('phobosLink is unpadded base64url with encoded fragment', () => {
    expect(phobosLink('x', 'Клиент 1')).toBe(
      'phobos://eA#%D0%9A%D0%BB%D0%B8%D0%B5%D0%BD%D1%82%201',
    );
    const long = phobosLink('a'.repeat(100), 'bob&co ?');
    expect(long).not.toContain('=');
    expect(long.endsWith('#bob%26co%20%3F')).toBe(true);
  });

  it('base64UrlNoPad is URL-safe', () => {
    expect(base64UrlNoPad('foobar')).toMatch(/^[-_A-Za-z0-9]+$/);
  });

  it('wgosocksClientConf golden matches the Go renderer', () => {
    expect(
      wgosocksClientConf('vpn.example.com', 51862, 'alice', 'pw1', {
        key: 'sk',
        masking: 'MEDIA',
        mediaSsrc: '305419896',
      }),
    ).toBe(
      [
        '[instance]',
        'mode = socks5',
        'role = client',
        'source-if = 127.0.0.1',
        'source-lport = 1080',
        'target = vpn.example.com:51862',
        'key = sk',
        'masking = MEDIA',
        'verbose = INFO',
        'media-ssrc = 305419896',
        '',
        '[socks5]',
        'login = alice',
        'password = pw1',
        '',
      ].join('\n'),
    );
  });

  it('wgosocksPhobosLink wraps the client conf', () => {
    const link = wgosocksPhobosLink('u1', 'h', 1, 'l', 'p', { key: 'k', masking: 'NONE' });
    expect(link.startsWith('phobos://')).toBe(true);
    expect(link.endsWith('#u1')).toBe(true);
  });
});
