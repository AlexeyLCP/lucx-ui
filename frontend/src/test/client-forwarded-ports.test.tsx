import { describe, it, expect, vi } from 'vitest';
import { render, fireEvent, waitFor, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';

import { ThemeProvider } from '@/hooks/useTheme';
import ClientFormModal from '@/pages/clients/ClientFormModal';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';

function makeQC() {
  return new QueryClient({ defaultOptions: { queries: { retry: false } } });
}

const AWG_INBOUND = {
  id: 7,
  port: 51820,
  protocol: 'awg',
  tag: 'awg-in',
  enable: true,
  awgVersion: '2',
} as unknown as InboundOption;

const CLIENT = {
  email: 'peer@x',
  subId: 'subpeer',
  enable: true,
  publicKey: 'pub',
  privateKey: 'priv',
  allowedIPs: '10.8.0.2/32',
  forwardedPorts: '',
} as unknown as ClientRecord;

describe('ClientFormModal — AWG forwarded ports', () => {
  it('sends the typed port-forward spec on save', async () => {
    const save = vi.fn().mockResolvedValue({ success: true });
    render(
      <ThemeProvider>
        <QueryClientProvider client={makeQC()}>
          <ClientFormModal
            open
            mode="edit"
            client={CLIENT}
            inbounds={[AWG_INBOUND]}
            attachedIds={[7]}
            save={save}
            onOpenChange={() => {}}
          />
        </QueryClientProvider>
      </ThemeProvider>,
    );

    fireEvent.click(await screen.findByRole('tab', { name: 'Credentials' }));
    const input = await screen.findByLabelText('Forwarded Ports');
    fireEvent.change(input, { target: { value: '8080' } });
    fireEvent.click(screen.getByRole('button', { name: /save/i }));

    await waitFor(() => expect(save).toHaveBeenCalled());
    expect((save.mock.calls[0][0] as Record<string, unknown>).forwardedPorts).toBe('8080');
  });
});
