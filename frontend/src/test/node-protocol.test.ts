import { describe, expect, it } from 'vitest';

import {
  filterNodesForProtocol,
  isLucxOnlyProtocol,
  isProtocolNodeEligible,
  nodeSupportsProtocol,
} from '@/lib/xray/node-protocol';
import type { NodeRecord } from '@/schemas/node';

function node(partial: Partial<NodeRecord> & { id: number }): NodeRecord {
  return {
    enable: true,
    status: 'online',
    ...partial,
  } as NodeRecord;
}

describe('node-protocol', () => {
  it('marks upstream and LucX protocols as node-eligible', () => {
    expect(isProtocolNodeEligible('vless')).toBe(true);
    expect(isProtocolNodeEligible('awg')).toBe(true);
    expect(isProtocolNodeEligible('naive')).toBe(true);
    expect(isProtocolNodeEligible('mtproto')).toBe(false);
    expect(isLucxOnlyProtocol('awg')).toBe(true);
    expect(isLucxOnlyProtocol('vless')).toBe(false);
  });

  it('allows upstream protocols on any node', () => {
    const vanilla = node({ id: 1, nodeType: 'vanilla', nodeFeatures: [] });
    expect(nodeSupportsProtocol(vanilla, 'vless')).toBe(true);
    expect(nodeSupportsProtocol(vanilla, 'awg')).toBe(false);
  });

  it('requires LucX feature for AWG', () => {
    const lucx = node({ id: 2, nodeType: 'lucx', nodeFeatures: ['awg', 'naive'] });
    expect(nodeSupportsProtocol(lucx, 'awg')).toBe(true);
    expect(nodeSupportsProtocol(lucx, 'qwdtt')).toBe(false);
  });

  it('treats nodeType lucx with empty features as all LucX protocols', () => {
    const lucx = node({ id: 3, nodeType: 'lucx', nodeFeatures: [] });
    expect(nodeSupportsProtocol(lucx, 'awg')).toBe(true);
    expect(nodeSupportsProtocol(lucx, 'qwdtt')).toBe(true);
  });

  it('falls back to panelVersion containing lucx', () => {
    const old = node({ id: 4, panelVersion: '3.6.0-lucx.100' });
    expect(nodeSupportsProtocol(old, 'awg')).toBe(true);
  });

  it('filters deploy targets', () => {
    const nodes = [
      node({ id: 1, name: 'vanilla', nodeType: 'vanilla' }),
      node({ id: 2, name: 'lucx', nodeType: 'lucx', nodeFeatures: ['awg'] }),
      node({ id: 3, name: 'off', enable: false, nodeType: 'lucx', nodeFeatures: ['awg'] }),
      node({ id: 4, name: 'hop', transitive: true, nodeType: 'lucx', nodeFeatures: ['awg'] }),
    ];
    const awg = filterNodesForProtocol(nodes, 'awg');
    expect(awg.map((n) => n.id)).toEqual([2]);
    const vless = filterNodesForProtocol(nodes, 'vless');
    expect(vless.map((n) => n.id)).toEqual([1, 2]);
  });
});
