import { describe, expect, it } from 'vitest';

import { buildClonePayload, pickClonePort } from '@/lib/xray/inbound-clone';
import { DBInbound } from '@/models/dbinbound';

function sourceInbound() {
  return new DBInbound({
    id: 7,
    port: 443,
    listen: '0.0.0.0',
    protocol: 'vless',
    remark: 'edge',
    enable: true,
    settings: JSON.stringify({
      clients: [{ id: 'uuid-1', email: 'a@test', flow: 'xtls-rprx-vision' }],
      decryption: 'none',
    }),
    streamSettings: {
      network: 'tcp',
      security: 'reality',
      realitySettings: { dest: 'www.lovelive-anime.jp:443' },
    },
    sniffing: { enabled: true },
    nodeId: 2,
    shareAddrStrategy: 'node',
    shareAddr: '',
  });
}

describe('buildClonePayload', () => {
  it('omits nodeId for a local-panel target so the row stays panel-local', () => {
    const payload = buildClonePayload(sourceInbound(), 23456, null);
    expect(payload).not.toHaveProperty('nodeId');
  });

  it('carries nodeId for a node target', () => {
    const payload = buildClonePayload(sourceInbound(), 23456, 5);
    expect(payload.nodeId).toBe(5);
  });

  it('stages the clone disabled with cleared clients, fresh port, and no tag', () => {
    const payload = buildClonePayload(sourceInbound(), 23456, 3);
    expect(payload.enable).toBe(false);
    expect(payload.port).toBe(23456);
    expect(payload.listen).toBe('');
    expect(payload).not.toHaveProperty('tag');
    expect(payload.remark).toBe('edge (clone)');

    const settings = JSON.parse(payload.settings);
    // Clients are dropped (emails are unique panel-wide, UUIDs must not
    // repeat across nodes) while the rest of the settings survive verbatim.
    expect(settings.clients).toEqual([]);
    expect(settings.decryption).toBe('none');
  });

  it('stringifies object-shaped streamSettings and sniffing from hydrated rows', () => {
    const payload = buildClonePayload(sourceInbound(), 23456, null);
    expect(JSON.parse(payload.streamSettings)).toEqual({
      network: 'tcp',
      security: 'reality',
      realitySettings: { dest: 'www.lovelive-anime.jp:443' },
    });
    expect(JSON.parse(payload.sniffing)).toEqual({ enabled: true });
  });

  it('survives malformed settings JSON with an empty client list fallback', () => {
    const broken = sourceInbound();
    broken.settings = '{not json';
    const payload = buildClonePayload(broken, 23456, null);
    const settings = JSON.parse(payload.settings);
    expect(settings.clients ?? []).toEqual([]);
  });
});

describe('pickClonePort', () => {
  it('never returns a port already bound on the target', () => {
    const used = new Set<number>();
    for (let p = 10000; p <= 60000; p++) if (p !== 23456) used.add(p);
    expect(pickClonePort(used)).toBe(23456);
  });

  it('stops probing when the range looks exhausted instead of spinning', () => {
    const used = new Set<number>();
    for (let p = 10000; p <= 60000; p++) used.add(p);
    const port = pickClonePort(used);
    expect(port).toBeGreaterThanOrEqual(10000);
    expect(port).toBeLessThanOrEqual(60000);
  });
});

describe('buildClonePayload — mieru binding remap (issue #218)', () => {
  function mieruSource(settings: unknown) {
    return new DBInbound({
      id: 21,
      port: 32893,
      listen: '0.0.0.0',
      protocol: 'mieru',
      remark: 'Mieru1NLv',
      enable: true,
      settings: JSON.stringify(settings),
      nodeId: 0,
      shareAddrStrategy: 'node',
      shareAddr: '',
    });
  }

  it('shifts a single binding off the source port when a used set is given', () => {
    const src = mieruSource({ portBindings: [{ port: 32893, protocol: 'TCP' }] });
    const used = new Set<number>([32893, 32894, 40000]);
    const payload = buildClonePayload(src, 23456, null, used);
    const settings = JSON.parse(payload.settings);
    expect(settings.portBindings[0].port).toBe(32895);
    expect(settings.portBindings[0].protocol).toBe('TCP');
  });

  it('keeps the span of a remapped port range and preserves a UDP entry', () => {
    const src = mieruSource({
      portBindings: [
        { portRange: '32893-32902', protocol: 'TCP' },
        { port: 33000, protocol: 'UDP' },
      ],
    });
    const used = new Set([32902, 32903]);
    const payload = buildClonePayload(src, 23456, null, used);
    const settings = JSON.parse(payload.settings);
    const [tcp, udp] = settings.portBindings;
    const [, end] = tcp.portRange.split('-').map(Number);
    expect(end - Number(tcp.portRange.split('-')[0]) + 1).toBe(10);
    expect(tcp.protocol).toBe('TCP');
    expect(udp.port).toBeGreaterThan(33000);
    expect(udp.protocol).toBe('UDP');
  });

  it('never lands a remapped binding on the source bindings', () => {
    const src = mieruSource({
      portBindings: [
        { port: 32893, protocol: 'TCP' },
        { port: 32894, protocol: 'TCP' },
      ],
    });
    // Target bound ports do not know the source's second binding (the
    // pre-check set is row-port only): the remap must block both originals.
    const used = new Set<number>([32893]);
    const payload = buildClonePayload(src, 23456, null, used);
    const settings = JSON.parse(payload.settings);
    const ports = settings.portBindings.map((b: { port: number }) => b.port);
    expect(ports).not.toContain(32893);
    expect(ports).not.toContain(32894);
    expect(new Set(ports).size).toBe(2);
  });

  it('leaves settings verbatim without a used set (legacy callers)', () => {
    const src = mieruSource({ portBindings: [{ port: 32893, protocol: 'TCP' }] });
    const payload = buildClonePayload(src, 23456, null);
    const settings = JSON.parse(payload.settings);
    expect(settings.portBindings[0].port).toBe(32893);
  });
});
