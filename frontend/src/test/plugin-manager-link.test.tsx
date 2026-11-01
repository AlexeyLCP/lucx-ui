import { screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { MemoryRouter } from 'react-router';
import { beforeAll, describe, expect, it, vi } from 'vitest';

import PluginManagerPage from '@/pages/plugins/PluginManagerPage';
import { renderWithProviders } from './test-utils';
import { HttpUtil, Msg } from '@/utils';

vi.mock('@/api/queries/useAllSettings', () => ({
  useAllSettings: () => ({ allSetting: {} }),
}));

beforeAll(async () => {
  const { readyI18n } = await import('@/i18n/react');
  await readyI18n('panel');
});

describe('plugin install by link', () => {
  it('previews the manifest and installs after the confirm click', async () => {
    const user = userEvent.setup();
    const postMock = vi
      .spyOn(HttpUtil, 'post')
      .mockResolvedValueOnce(
        new Msg(
          true,
          '',
          JSON.parse(
            '{"id":"hello","name":"Hello","version":"1.0.0","apiVersion":1,"type":"sidecar","entrypoint":"run.sh","permissions":{"network":["none"]}}',
          ),
        ),
      )
      .mockResolvedValueOnce(new Msg(true, '', null));
    vi.spyOn(HttpUtil, 'get').mockImplementation(async () => new Msg(true, '', []));

    renderWithProviders(
      <MemoryRouter>
        <PluginManagerPage />
      </MemoryRouter>,
    );
    // href="#" turns the Button into an anchor: click it by text, not role.
    await user.click(await screen.findByText('Install from link'));
    const dialog = await screen.findByRole('dialog');
    const input = dialog.querySelector('input') as HTMLInputElement;
    if (!input) throw new Error('link input not found');
    await user.type(input, 'github.com/owner/repo');
    await user.click(await screen.findByRole('button', { name: 'Check' }));
    await waitFor(() => expect(screen.getByText('hello')).toBeTruthy());
    expect(postMock.mock.calls[0][0]).toContain('/install/url/preview');
    await user.click(await screen.findByRole('button', { name: 'Install' }));
    await waitFor(() => expect(postMock).toHaveBeenCalledTimes(2));
    expect(postMock.mock.calls[1][0]).toContain('/install/url');
    expect(postMock.mock.calls[1][1]).toEqual({ url: 'github.com/owner/repo' });
  });
});
