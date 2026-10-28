// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { inflateRawSync } from 'node:zlib';
import { describe, expect, it } from 'vitest';

import { encodeOpenfluxLink } from '@/lib/xray/openflux-link';
import { genOpenfluxLink } from '@/lib/xray/inbound-link';
import type { Inbound } from '@/schemas/api/inbound';

function decode(link: string): string {
  const body = link.slice('openflux://v1/'.length);
  const b64 = body.replace(/-/g, '+').replace(/_/g, '/');
  const padded = b64 + '='.repeat((4 - (b64.length % 4)) % 4);
  const raw = Buffer.from(padded, 'base64');
  return inflateRawSync(raw).toString('utf8');
}

describe('encodeOpenfluxLink', () => {
  it('emits a stored-block openflux://v1 link', () => {
    const link = encodeOpenfluxLink({
      secret: '0123456789abcdef',
      host: '203.0.113.5',
      port: 18445,
      remark: 'exit',
      yandexUrl: 'https://docs.example/view?id=1',
    });
    expect(link.startsWith('openflux://v1/')).toBe(true);
    const payload = JSON.parse(decode(link));
    expect(payload.negotiate).toBe(true);
    expect(payload.context).toBe('https://docs.example/view?id=1');
    expect(payload.transports[0]).toEqual({
      type: 'direct',
      priority: 100,
      dial: '203.0.113.5:18445',
    });
    expect(payload.transports[1].type).toBe('yandex');
  });

  it('refuses a short secret', () => {
    expect(encodeOpenfluxLink({ secret: 'short', host: '1.2.3.4', port: 1 })).toBe('');
  });

  it('genOpenfluxLink prefers shareHost', () => {
    const inbound = {
      protocol: 'openflux',
      port: 18445,
      remark: 'n',
      settings: { secret: '0123456789abcdef', shareHost: '9.9.9.9' },
    } as Inbound;
    const payload = JSON.parse(decode(genOpenfluxLink({ inbound, address: '1.1.1.1' })));
    expect(payload.transports[0].dial).toBe('9.9.9.9:18445');
  });
});

// LUCX-HOOK (#148): carriers shareable since core v0.13.0 — same priorities
// and order as the Go side, original transports untouched.
describe('encodeOpenfluxLink v0.13.0 carriers', () => {
  it('carries vyandex/telemost/boards/avito in the panel order under direct', () => {
    const link = encodeOpenfluxLink({
      secret: '0123456789abcdef',
      host: '203.0.113.5',
      port: 18445,
      vyandexUrl: 'https://docs.yandex.ru/edit/d/abcdefghijklmnop',
      telemostUrl: 'https://telemost.yandex.ru/j/123456',
      boardsUrl: 'https://boards.yandex.ru/whiteboard/?hash=0123456789abcdef0123456789abcdef',
      avitoUrl: 'abcdefghij',
    });
    const payload = JSON.parse(decode(link));
    expect(payload.context).toBe('https://docs.yandex.ru/edit/d/abcdefghijklmnop');
    expect(payload.transports.map((t: { type: string }) => t.type)).toEqual([
      'direct',
      'vyandex',
      'telemost',
      'boards',
      'avito',
    ]);
    expect(payload.transports[1].priority).toBe(52);
    expect(payload.transports[2].priority).toBe(35);
    expect(payload.transports[3].priority).toBe(32);
    expect(payload.transports[4].priority).toBe(31);
  });

  it('keeps the legacy three-carrier link byte-identical in shape', () => {
    const link = encodeOpenfluxLink({
      secret: '0123456789abcdef',
      host: '203.0.113.5',
      port: 18445,
      yandexUrl: 'https://docs.example/view?id=1',
      mailruUrl: 'https://cloud.mail.ru/public/a/b',
      cupsUrl: 'https://interview.cups.online/live-coding/?room=x',
    });
    const payload = JSON.parse(decode(link));
    expect(payload.transports.map((t: { type: string }) => t.type)).toEqual([
      'direct',
      'yandex',
      'mailru',
      'cupsonline',
    ]);
    expect(payload.transports.map((t: { priority: number }) => t.priority)).toEqual([
      100, 50, 40, 30,
    ]);
  });
});
