import { describe, it, expect } from 'vitest';
import { InboundFormSchema } from '@/schemas/forms/inbound-form';
import { createDefaultQwdttInboundSettings } from '@/lib/xray/inbound-defaults';
import { formValuesToWirePayload } from '@/lib/xray/inbound-form-adapter';

function base(over: Record<string, unknown> = {}) {
  return {
    remark: 'test',
    enable: true,
    port: 12345,
    listen: '',
    tag: '',
    expiryTime: 0,
    sniffing: {
      enabled: false,
      destOverride: ['http', 'tls', 'quic', 'fakedns'],
      metadataOnly: false,
      routeOnly: false,
      ipsExcluded: [],
      domainsExcluded: [],
    },
    streamSettings: { network: 'tcp', security: 'none', tcpSettings: {} },
    up: 0,
    down: 0,
    total: 0,
    trafficReset: 'never' as const,
    trafficResetDay: 1,
    lastTrafficResetTime: 0,
    nodeId: null,
    shareAddrStrategy: 'node' as const,
    shareAddr: '',
    subSortIndex: 1,
    protocol: 'qwdtt' as const,
    settings: createDefaultQwdttInboundSettings(),
    ...over,
  };
}

describe('qwdtt inbound form', () => {
  it('parses default add-mode shape', () => {
    const r = InboundFormSchema.safeParse(base());
    expect(r.success, JSON.stringify(r.success ? null : r.error.issues)).toBe(true);
  });

  it('parses transportless stream', () => {
    const r = InboundFormSchema.safeParse(base({ streamSettings: { security: 'none' } }));
    expect(r.success, JSON.stringify(r.success ? null : r.error.issues)).toBe(true);
  });

  it('parses without streamSettings', () => {
    const r = InboundFormSchema.safeParse(base({ streamSettings: undefined }));
    expect(r.success, JSON.stringify(r.success ? null : r.error.issues)).toBe(true);
  });

  it('rejects null InputNumber fields', () => {
    const settings = { ...createDefaultQwdttInboundSettings(), wgPort: null, workers: null };
    const r = InboundFormSchema.safeParse(base({ settings }));
    expect(r.success).toBe(false);
  });

  it('wire payload has protocol qwdtt and empty stream', () => {
    const parsed = InboundFormSchema.parse(base());
    const wire = formValuesToWirePayload(parsed);
    expect(wire.protocol).toBe('qwdtt');
    expect(JSON.parse(wire.settings).listenAddr).toBe('0.0.0.0:56000');
  });

  it('parses transportless stream after sidecar protocol switch shape', () => {
    const r = InboundFormSchema.safeParse(
      base({
        port: 56000,
        streamSettings: { security: 'none' },
      }),
    );
    expect(r.success, JSON.stringify(r.success ? null : r.error.issues)).toBe(true);
  });
});
