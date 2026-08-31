import { describe, it, expect, vi } from 'vitest';
import { screen, fireEvent, waitFor } from '@testing-library/react';

import InboundFormModal from '@/pages/inbounds/form/InboundFormModal';
import { DBInbound } from '@/models/dbinbound';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

const { messageError } = vi.hoisted(() => ({ messageError: vi.fn() }));

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    message: {
      ...actual.message,
      useMessage: () => [{ error: messageError }, null],
    },
  };
});

// 3596 chars = 3604 IBytes against a 3492 budget — the set from the field
// report, stored since lucx.190 when nothing measured it.
const OVERSIZE = 'x'.repeat(3596);

function legacyAwgInbound(): DBInbound {
  return new DBInbound({
    id: 7,
    port: 51820,
    listen: '',
    protocol: 'awg',
    remark: 'legacy awg',
    enable: true,
    settings: {
      privateKey: 'aBcD',
      publicKey: '',
      address: '10.200.0.1/24',
      mtu: 1420,
      obfLevel: 3,
      jc: 4,
      jmin: 10,
      jmax: 50,
      s1: 20,
      s2: 30,
      s3: 15,
      s4: 13,
      h1: '1',
      h2: '2',
      h3: '3',
      h4: '4',
      i1: OVERSIZE,
      headerProtectionKey: '',
      awgVersion: '2',
      clients: [],
    },
    streamSettings: { network: 'tcp', security: 'none', tcpSettings: {} },
    sniffing: { enabled: false },
    nodeId: null,
    shareAddrStrategy: 'listen',
    shareAddr: '',
  });
}

function renderEdit(dbInbound: DBInbound) {
  renderWithProviders(
    <InboundFormModal
      open
      mode="edit"
      dbInbound={dbInbound}
      dbInbounds={[dbInbound]}
      availableNodes={[]}
      onClose={() => {}}
      onSaved={() => {}}
    />,
  );
}

function inboundSaveCalls(): unknown[][] {
  return vi
    .mocked(HttpUtil.post)
    .mock.calls.filter((call) => String(call[0]).startsWith('/panel/api/inbounds/'));
}

function primaryButton(): HTMLElement {
  const button = document.querySelector('.ant-modal-footer .ant-btn-primary');
  if (!button) throw new Error('Primary modal button not found');
  return button as HTMLElement;
}

describe('InboundFormModal AWG I-field budget', () => {
  it('renames an inbound whose stored I-set has been over budget since lucx.190', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    messageError.mockClear();
    renderEdit(legacyAwgInbound());

    fireEvent.change(await screen.findByDisplayValue('legacy awg'), {
      target: { value: 'renamed' },
    });
    fireEvent.click(primaryButton());

    await waitFor(() => {
      expect(post).toHaveBeenCalledWith(
        '/panel/api/inbounds/update/7',
        expect.objectContaining({ remark: 'renamed' }),
      );
    });
    expect(messageError).not.toHaveBeenCalled();
  });

  it('refuses a different over-budget set typed into that same form', async () => {
    const post = vi.mocked(HttpUtil.post);
    post.mockClear();
    messageError.mockClear();
    renderEdit(legacyAwgInbound());

    fireEvent.change(await screen.findByDisplayValue(OVERSIZE), {
      target: { value: 'y'.repeat(3596) },
    });
    fireEvent.click(primaryButton());

    await waitFor(() => {
      expect(messageError).toHaveBeenCalledWith(expect.stringContaining('netlink read budget'));
    });
    expect(inboundSaveCalls()).toEqual([]);
  });
});
