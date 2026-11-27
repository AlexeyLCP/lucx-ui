/// <reference types="vite/client" />
import { describe, expect, it } from 'vitest';

import { InboundSettingsSchema } from '@/schemas/protocols';

// import.meta.glob (eager, default-import) gives us {path: parsedJson} at
// compile time — no fs, no @types/node. Vitest inherits the vite/client
// shape so this stays typed.
const inboundFixtures = import.meta.glob<unknown>('./golden/fixtures/inbound/*.json', {
  eager: true,
  import: 'default',
});

function fixtureName(path: string): string {
  const file = path.split('/').pop() ?? path;
  return file.replace(/\.json$/, '');
}

describe('InboundSettingsSchema fixtures', () => {
  const entries = Object.entries(inboundFixtures).sort(([a], [b]) => a.localeCompare(b));
  expect(
    entries.length,
    'expected at least one fixture under golden/fixtures/inbound',
  ).toBeGreaterThan(0);

  for (const [path, raw] of entries) {
    it(`parses ${fixtureName(path)} byte-stably`, () => {
      const parsed = InboundSettingsSchema.parse(raw);
      expect(parsed).toMatchSnapshot();
    });
  }
});

// The fixture tests above pin coerced values only via regenerable snapshots. These
// assert the load-bearing transforms directly, so a broken coercion fails independently
// of the snapshot baseline.
describe('InboundSettingsSchema coercions', () => {
  it('vmess: defaults alterId to 0 and coerces a string tgId to a number', () => {
    const parsed = InboundSettingsSchema.parse({
      protocol: 'vmess',
      settings: { clients: [{ id: 'u1', email: 'a@b.c', tgId: '12345' }] },
    });
    if (parsed.protocol !== 'vmess')
      throw new Error('discriminator narrowed to the wrong protocol');
    const client = parsed.settings.clients[0];
    expect(client.alterId).toBe(0); // .default(0) injected for omitted field
    expect(client.tgId).toBe(12345); // string -> number transform
  });

  it('vmess: a non-numeric tgId coerces to 0', () => {
    const parsed = InboundSettingsSchema.parse({
      protocol: 'vmess',
      settings: { clients: [{ id: 'u1', email: 'a@b.c', tgId: 'not-a-number' }] },
    });
    if (parsed.protocol !== 'vmess') throw new Error('wrong protocol');
    expect(parsed.settings.clients[0].tgId).toBe(0); // Number(v) || 0
  });

  it('vless: defaults decryption and encryption to "none"', () => {
    const parsed = InboundSettingsSchema.parse({ protocol: 'vless', settings: { clients: [] } });
    if (parsed.protocol !== 'vless') throw new Error('wrong protocol');
    expect(parsed.settings.decryption).toBe('none');
    expect(parsed.settings.encryption).toBe('none');
  });

  // lucx.342 fields must survive the zod round-trip the form save does
  // (InboundFormModal: safeParse -> formValuesToWirePayload). Missing from the
  // schema once, they were stripped and TrustTunnel kept answering 407.
  it('trusttunnel: keeps siteUpstream and authFailureStatus past parse', () => {
    const parsed = InboundSettingsSchema.parse({
      protocol: 'trusttunnel',
      settings: {
        hostname: 'vpn.example.com',
        siteUpstream: '127.0.0.1:8081',
        authFailureStatus: 405,
      },
    });
    if (parsed.protocol !== 'trusttunnel') throw new Error('wrong protocol');
    expect(parsed.settings.siteUpstream).toBe('127.0.0.1:8081');
    expect(parsed.settings.authFailureStatus).toBe(405);
  });

  it('trusttunnel: defaults siteUpstream to empty and authFailureStatus to 0 (407)', () => {
    const parsed = InboundSettingsSchema.parse({
      protocol: 'trusttunnel',
      settings: { hostname: 'vpn.example.com' },
    });
    if (parsed.protocol !== 'trusttunnel') throw new Error('wrong protocol');
    expect(parsed.settings.siteUpstream).toBe('');
    expect(parsed.settings.authFailureStatus).toBe(0);
  });

  // lucx.369 report (Андрей): the anytls «Сайт для неавторизованных» field
  // reopened empty after save — fallback was missing from this schema and got
  // stripped in both directions.
  it('anytls: keeps fallback past parse', () => {
    const parsed = InboundSettingsSchema.parse({
      protocol: 'anytls',
      settings: { port: 8443, fallback: '127.0.0.1:443' },
    });
    if (parsed.protocol !== 'anytls') throw new Error('wrong protocol');
    expect(parsed.settings.fallback).toBe('127.0.0.1:443');
  });

  it('anytls: defaults fallback to empty', () => {
    const parsed = InboundSettingsSchema.parse({
      protocol: 'anytls',
      settings: { port: 8443 },
    });
    if (parsed.protocol !== 'anytls') throw new Error('wrong protocol');
    expect(parsed.settings.fallback).toBe('');
  });

  // lucx.369 report (Андрей): cover LocalPort / LoginPage existed only in Go
  // and were stripped on save — the local HTTP site (anytls fallback target)
  // and sign-in mode never persisted.
  it('cover: keeps localPort and loginPage past parse', () => {
    const parsed = InboundSettingsSchema.parse({
      protocol: 'cover',
      settings: { hostname: 'vpn.example.com', localPort: 3443, loginPage: true },
    });
    if (parsed.protocol !== 'cover') throw new Error('wrong protocol');
    expect(parsed.settings.localPort).toBe(3443);
    expect(parsed.settings.loginPage).toBe(true);
  });

  it('cover: defaults localPort to 0 and loginPage to false', () => {
    const parsed = InboundSettingsSchema.parse({
      protocol: 'cover',
      settings: { hostname: 'vpn.example.com' },
    });
    if (parsed.protocol !== 'cover') throw new Error('wrong protocol');
    expect(parsed.settings.localPort).toBe(0);
    expect(parsed.settings.loginPage).toBe(false);
  });
});
