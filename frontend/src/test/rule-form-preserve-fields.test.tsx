import { afterEach, describe, it, expect, vi } from 'vitest';
import { cleanup, fireEvent, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import RuleFormModal from '@/pages/xray/routing/RuleFormModal';
import { HttpUtil, Msg } from '@/utils';

import { chooseSelectOption, listSelectOptions, renderWithProviders } from './test-utils';

describe('RuleFormModal edit preserves unsurfaced fields', () => {
  afterEach(() => vi.restoreAllMocks());

  it('keeps a field the form does not surface (ruleTag) when saving an edit', async () => {
    const onConfirm = vi.fn();
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false, gcTime: 0 } },
    });

    renderWithProviders(
      <QueryClientProvider client={queryClient}>
        <RuleFormModal
          open
          rule={{
            type: 'field',
            outboundTag: 'block',
            ruleTag: 'my-tag',
            enabled: true,
            domain: ['example.com'],
          }}
          inboundTags={[]}
          outboundTags={['block']}
          balancerTags={[]}
          onClose={vi.fn()}
          onConfirm={onConfirm}
        />
      </QueryClientProvider>,
    );

    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    expect(onConfirm).toHaveBeenCalledTimes(1);
    expect(onConfirm.mock.calls[0][0]).toMatchObject({ ruleTag: 'my-tag' });

    // Drain pending client-list query so unmount does not race react-dom.
    await queryClient.cancelQueries();
    cleanup();
    queryClient.clear();
  });

  // LUCX-HOOK: the fork replaces upstream's email-only user select with the
  // client picker (#clientPick): clients map to user:/src:/in: options, the
  // query key is ['routing','clientPickList'], and free tags use user:<id>.
  it('selects existing clients for the user criterion', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      new Msg(true, '', [
        { id: 1, email: 'alice@example.com' },
        { id: 2, email: 'bob@example.com' },
      ]),
    );
    const onConfirm = vi.fn();
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    renderWithProviders(
      <RuleFormModal
        open
        rule={null}
        inboundTags={[]}
        outboundTags={['direct']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
      { queryClient },
    );

    await waitFor(() =>
      expect(HttpUtil.get).toHaveBeenCalledWith('/panel/api/clients/list', undefined, {
        silent: true,
      }),
    );
    await waitFor(() =>
      expect(queryClient.getQueryData(['routing', 'clientPickList'])).toHaveLength(2),
    );
    chooseSelectOption('clientPick', 'alice@example.com · xray');
    chooseSelectOption('clientPick', 'bob@example.com · xray');
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    expect(onConfirm).toHaveBeenCalledWith(
      expect.objectContaining({ user: ['alice@example.com', 'bob@example.com'] }),
    );
  });

  it('keeps every AWG client distinct in the picker (no virtual-list reuse)', async () => {
    const clients = Array.from({ length: 40 }, (_, i) => ({
      id: i + 1,
      email: `peer_${String(i + 1).padStart(2, '0')}`,
      inboundIds: [1],
      allowedIPs: `10.200.2.${i + 1}/32`,
    }));
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url) => {
      if (String(url).includes('inbounds/options')) {
        return new Msg(true, '', [{ id: 1, protocol: 'awg', tag: 'awg-1' }]);
      }
      return new Msg(true, '', clients);
    });
    const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });

    renderWithProviders(
      <RuleFormModal
        open
        rule={null}
        inboundTags={[]}
        outboundTags={['direct']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={vi.fn()}
      />,
      { queryClient },
    );

    await waitFor(() =>
      expect(queryClient.getQueryData(['routing', 'clientPickList'])).toHaveLength(40),
    );
    await waitFor(() => expect(queryClient.getQueryData(['inbounds', 'options'])).toBeTruthy());

    const labels = listSelectOptions('clientPick');
    expect(document.querySelector('.rc-virtual-list')).toBeNull();
    expect(new Set(labels).size).toBe(40);
    expect(labels).toContain('peer_01 · AWG · 10.200.2.1/32');
    expect(labels).toContain('peer_40 · AWG · 10.200.2.40/32');
  });
  // END LUCX-HOOK

  it('preserves a saved user that is no longer in the client list', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      new Msg(true, '', [{ id: 1, email: 'alice@example.com' }]),
    );
    const onConfirm = vi.fn();

    renderWithProviders(
      <RuleFormModal
        open
        rule={{ type: 'field', user: ['removed@example.com'], outboundTag: 'direct' }}
        inboundTags={[]}
        outboundTags={['direct']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    // LUCX-HOOK: saved users reappear as user: tags in the picker, not as
    // plain text next to an input.
    await waitFor(() => expect(screen.getByText('user:removed@example.com')).toBeTruthy());
    // END LUCX-HOOK
    fireEvent.click(screen.getByRole('button', { name: 'Save Changes' }));

    expect(onConfirm).toHaveBeenCalledWith(
      expect.objectContaining({ user: ['removed@example.com'] }),
    );
  });

  it('accepts a custom user identifier that is not a client record', async () => {
    vi.spyOn(HttpUtil, 'get').mockResolvedValue(
      new Msg(true, '', [{ id: 1, email: 'alice@example.com' }]),
    );
    const onConfirm = vi.fn();

    renderWithProviders(
      <RuleFormModal
        open
        rule={null}
        inboundTags={[]}
        outboundTags={['direct']}
        balancerTags={[]}
        onClose={vi.fn()}
        onConfirm={onConfirm}
      />,
    );

    // LUCX-HOOK: free tags typed as user:<id> land in the user criterion.
    const user = userEvent.setup();
    const picker = document.getElementById('clientPick') as HTMLInputElement;
    await user.click(picker);
    await user.type(picker, 'user:office-proxy');
    const typedOption = Array.from(document.querySelectorAll('.ant-select-item-option')).find(
      (o) =>
        o.getAttribute('title') === 'user:office-proxy' || o.textContent === 'user:office-proxy',
    );
    expect(typedOption).toBeTruthy();
    await user.click(typedOption as HTMLElement);
    // END LUCX-HOOK
    fireEvent.click(screen.getByRole('button', { name: 'Create' }));

    expect(onConfirm).toHaveBeenCalledWith(expect.objectContaining({ user: ['office-proxy'] }));
  });
});
