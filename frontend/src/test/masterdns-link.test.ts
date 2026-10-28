// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { describe, expect, it } from 'vitest';

import { genMasterdnsLink, resolversFile } from '@/lib/xray/masterdns-link';

describe('genMasterdnsLink', () => {
  it('wraps the client TOML in a masterdns://v1 base64url envelope', () => {
    const link = genMasterdnsLink({
      inbound: {
        settings: {
          domains: 'V.Example.com, d2.example.com',
          secret: '0123456789abcdef0123456789abcdef',
          encryption: 2,
        },
      },
    });
    expect(link.startsWith('masterdns://v1/')).toBe(true);
    const body = link.slice('masterdns://v1/'.length);
    const b64 = body.replace(/-/g, '+').replace(/_/g, '/');
    const text = Buffer.from(b64 + '='.repeat((4 - (b64.length % 4)) % 4), 'base64').toString(
      'utf8',
    );
    expect(text).toContain('DOMAINS = ["v.example.com", "d2.example.com"]');
    expect(text).toContain('DATA_ENCRYPTION_METHOD = 2');
    expect(text).toContain('ENCRYPTION_KEY = "0123456789abcdef0123456789abcdef"');
    expect(text).toContain('RESOLVERS_FILE_PATH = "client_resolvers.txt"');
  });

  it('refuses a short secret or empty domains', () => {
    expect(
      genMasterdnsLink({ inbound: { settings: { domains: 'v.example.com', secret: 'abc' } } }),
    ).toBe('');
    expect(
      genMasterdnsLink({
        inbound: { settings: { domains: '', secret: '0123456789abcdef0123456789abcdef' } },
      }),
    ).toBe('');
  });

  it('renders the operator resolver list or the public defaults', () => {
    expect(resolversFile('9.9.9.9:53, 1.0.0.1:53')).toBe('9.9.9.9:53\n1.0.0.1:53\n');
    expect(resolversFile(undefined)).toContain('8.8.8.8:53');
  });
});
