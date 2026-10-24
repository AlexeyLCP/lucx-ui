// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';
import { FormProvider, useForm, useWatch } from 'react-hook-form';
import { Form } from 'antd';
import WireguardFields from '@/pages/inbounds/form/protocols/wireguard';
import { HttpUtil } from '@/utils';
import { renderWithProviders } from './test-utils';

function Harness() {
  const form = useForm({
    defaultValues: {
      enable: true,
      port: 51820,
      listen: '',
      settings: { kernel: true, address: '10.90.0.1/24', tproxyPort: 32101, secretKey: 'old' },
    },
  });
  const values = useWatch({ control: form.control });
  return (
    <FormProvider {...form}>
      <Form>
        <WireguardFields wgPubKey="" regenInboundWg={() => {}} />
        <output data-testid="state">{JSON.stringify(values)}</output>
      </Form>
    </FormProvider>
  );
}

describe('kernel WireGuard import preview', () => {
  it('accepts the nested settings object emitted by the inbound API and keeps the draft disabled', async () => {
    const post = vi.spyOn(HttpUtil, 'post').mockResolvedValue({
      success: true,
      msg: '',
      obj: {
        port: 13531,
        enable: false,
        settings: {
          kernel: true,
          address: '10.91.0.1/24',
          tproxyPort: 32101,
          secretKey: 'imported',
          clients: [],
        },
      },
    });
    renderWithProviders(<Harness />);
    fireEvent.click(screen.getByRole('button', { name: 'Import wg-quick server config' }));
    fireEvent.change(document.querySelector('textarea')!, { target: { value: '[Interface]' } });
    fireEvent.click(screen.getByRole('button', { name: 'OK' }));
    await waitFor(() =>
      expect(screen.getByTestId('state').textContent).toContain('"secretKey":"imported"'),
    );
    expect(screen.getByTestId('state').textContent).toContain('"enable":false');
    expect(screen.getByTestId('state').textContent).toContain('"port":13531');
    expect(post).toHaveBeenCalledTimes(1);
  });
});
