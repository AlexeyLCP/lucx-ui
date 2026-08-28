import { describe, expect, it } from 'vitest';

import { awgIBytes, awgWorstCaseIBytesBudget } from '@/lib/xray/awg-budget';
import { genAwgConfig } from '@/lib/xray/inbound-link';
import type { AwgInboundSettings } from '@/schemas/protocols/inbound/awg';

function awgSettings(over: Partial<AwgInboundSettings> = {}): AwgInboundSettings {
  return {
    privateKey: 'serverPrivKeyBase64',
    publicKey: '',
    address: '10.8.0.1/24',
    mtu: 1320,
    obfLevel: 3,
    mimicryProfile: 'quic',
    browserProfile: 'chrome',
    region: 'world',
    jc: 5,
    jmin: 50,
    jmax: 200,
    s1: 30,
    s2: 60,
    s3: 20,
    s4: 25,
    h1: '100000-500000',
    h2: '600000-900000',
    h3: '1000000-1500000',
    h4: '1600000-2000000',
    i1: '',
    i2: '',
    i3: '',
    i4: '',
    i5: '',
    headerProtectionKey: '',
    awgVersion: '2',
    contentPaddingAddition: '0',
    rekeyAfterTime: '0',
    rekeyTimeout: '0',
    rejectAfterTime: '0',
    keepaliveTimeout: '0',
    maxHandshakeAttempts: '0',
    randomTrailers: false,
    disableCookies: false,
    routeThroughXray: true,
    outboundTag: '',
    clients: [
      {
        privateKey: 'clientPrivKeyBase64',
        publicKey: 'peerPub',
        preSharedKey: 'psk',
        allowedIPs: ['10.8.0.2/32'],
        keepAlive: '25',
        email: 'u',
        limitIp: 0,
        totalGB: 0,
        expiryTime: 0,
        enable: true,
        tgId: 0,
        subId: '',
        comment: '',
        reset: 0,
      },
    ] as AwgInboundSettings['clients'],
    ...over,
  };
}

function conf(over: Partial<AwgInboundSettings> = {}): string {
  return genAwgConfig({
    settings: awgSettings(over),
    address: 'wg.example.test',
    port: 51820,
    peerIndex: 0,
  });
}

// An I-set over the netlink read budget vanishes from the live interface, so
// the exported .conf must drop it whole — same gate as the four Go renderers.
describe('genAwgConfig I-field budget', () => {
  it('keeps every field when the set lands exactly on the budget', () => {
    const big = 'x'.repeat(3452); // 3460 bytes + 4 x 8 = 3492, the worst-case budget
    const txt = conf({ i1: big, i2: 'a', i3: 'a', i4: 'a', i5: 'a' });
    expect(txt).toContain(`I1 = ${big}\n`);
    expect(txt).toContain('I2 = a\n');
    expect(txt).toContain('I5 = a\n');
  });

  it('drops all five fields one align step over the budget', () => {
    const big = 'x'.repeat(3456); // 3464 bytes + 4 x 8 = 3496, one step over
    const txt = conf({ i1: big, i2: 'a', i3: 'a', i4: 'a', i5: 'a' });
    expect(txt).not.toContain('I1 = ');
    expect(txt).not.toContain('I2 = ');
    expect(txt).not.toContain('I3 = ');
    expect(txt).not.toContain('I4 = ');
    expect(txt).not.toContain('I5 = ');
  });

  it('agrees with the Go .conf renderer on its two pinned sample sizes', () => {
    const fits = 'x'.repeat(3484); // IBytes 3492 in TestRenderClientConf_IFieldsBudget
    const over = 'x'.repeat(3488); // IBytes 3496
    expect(conf({ i1: fits })).toContain(`I1 = ${fits}\n`);
    expect(conf({ i1: over })).not.toContain('I1 = ');
  });

  it('charges a header protection key the 36 bytes Go charges it', () => {
    const v = 'x'.repeat(3463); // IBytes 3468: fits 3492, not 3456
    expect(conf({ i1: v })).toContain(`I1 = ${v}\n`);
    expect(conf({ i1: v, headerProtectionKey: 'aBcD...base64hpk==' })).not.toContain('I1 = ');
  });

  it('measures and writes the trimmed value, as Go does', () => {
    const v = 'x'.repeat(3484);
    const txt = conf({ i1: `  ${v}  `, i2: '   ' });
    expect(txt).toContain(`I1 = ${v}\n`);
    expect(txt).not.toContain('I2 = ');
  });
});

// Pins against drift from internal/awg/cps_budget.go — the sample values come
// from TestWorstCaseIBytesBudget, TestIBytes and its non-monotonic sibling.
describe('awg-budget arithmetic matches internal/awg/cps_budget.go', () => {
  it('budgets 3492 bytes, or 3456 once a header protection key claims its slot', () => {
    expect(awgWorstCaseIBytesBudget(false)).toBe(3492);
    expect(awgWorstCaseIBytesBudget(true)).toBe(3456);
  });

  it('charges every non-empty field one aligned netlink slot', () => {
    expect(awgIBytes('', '', '', '', '')).toBe(0);
    expect(awgIBytes('a', '', '', '', '')).toBe(8);
    expect(awgIBytes('aaaaaaa', '', '', '', '')).toBe(12);
    expect(awgIBytes('aaaaaaaa', '', '', '', '')).toBe(16);
    expect(awgIBytes('aaaa', '', '  ', '', '')).toBe(12);
  });

  it('is not monotonic in the character sum', () => {
    expect(awgIBytes('a'.repeat(10), '', '', '', '')).toBe(16);
    expect(awgIBytes('a'.repeat(5), 'a'.repeat(5), '', '', '')).toBe(24);
  });

  it('counts UTF-8 bytes, not UTF-16 units, as Go len() does', () => {
    expect(awgIBytes('привет', '', '', '', '')).toBe(20);
  });
});
