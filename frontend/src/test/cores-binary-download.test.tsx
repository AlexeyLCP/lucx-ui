import { render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { QueryClientProvider } from '@tanstack/react-query';
import { MemoryRouter } from 'react-router';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

import CoresTab from '@/pages/settings/CoresTab';
import { makeTestQueryClient } from '@/test/test-utils';
import { HttpUtil, Msg } from '@/utils';

const VALID_SHA = 'a'.repeat(64);
const BINARY_URL = 'https://example.com/caddy-naive-linux-amd64';

/*
 * The assertions go through HttpUtil rather than the tunnelsApi helpers: the
 * Cores page captures those helpers into a module-level table at import time,
 * so spying on the module object would leave the captured references in place
 * and pass no matter what the component does. Checking the request body also
 * pins the wire format the Go handler binds to.
 */
function downloadCalls(post: ReturnType<typeof spyOnPost>) {
  return post.mock.calls.filter(([url]) => String(url).includes('/download'));
}

function spyOnPost() {
  return vi.spyOn(HttpUtil, 'post').mockImplementation(async () => new Msg(true, '', null));
}

function renderCores() {
  return render(
    <MemoryRouter>
      <QueryClientProvider client={makeTestQueryClient()}>
        <CoresTab />
      </QueryClientProvider>
    </MemoryRouter>,
  );
}

async function openFirstDownloadDialog() {
  const user = userEvent.setup();
  const buttons = await screen.findAllByRole('button', { name: /download/i });
  await user.click(buttons[0]);
  const dialog = await screen.findByRole('dialog');
  const [urlInput, shaInput] = within(dialog).getAllByRole('textbox');
  const confirm = within(dialog).getByRole('button', { name: /download/i });
  return { user, urlInput, shaInput, confirm };
}

beforeEach(() => {
  vi.spyOn(HttpUtil, 'get').mockImplementation(async () => new Msg(true, '', {
    core: 'naive',
    displayName: 'NaiveProxy',
    binaryExists: false,
    binaryPath: 'bin/caddy-naive-linux-amd64',
    probe: { running: false },
  }));
});

afterEach(() => {
  vi.restoreAllMocks();
});

describe('core binary download', () => {
  it('omits the checksum when the field is left empty', async () => {
    const post = spyOnPost();
    renderCores();

    const { user, urlInput, confirm } = await openFirstDownloadDialog();
    await user.type(urlInput, BINARY_URL);
    await user.click(confirm);

    await waitFor(() => {
      expect(downloadCalls(post)).toHaveLength(1);
    });
    expect(downloadCalls(post)[0][1]).toEqual({ url: BINARY_URL, sha256: undefined });
  });

  it('forwards a well-formed checksum alongside the url', async () => {
    const post = spyOnPost();
    renderCores();

    const { user, urlInput, shaInput, confirm } = await openFirstDownloadDialog();
    await user.type(urlInput, BINARY_URL);
    await user.type(shaInput, VALID_SHA);
    await user.click(confirm);

    await waitFor(() => {
      expect(downloadCalls(post)).toHaveLength(1);
    });
    expect(downloadCalls(post)[0][1]).toEqual({ url: BINARY_URL, sha256: VALID_SHA });
  });

  it('accepts an uppercase checksum', async () => {
    const post = spyOnPost();
    renderCores();

    const { user, urlInput, shaInput, confirm } = await openFirstDownloadDialog();
    await user.type(urlInput, BINARY_URL);
    await user.type(shaInput, VALID_SHA.toUpperCase());
    await user.click(confirm);

    await waitFor(() => {
      expect(downloadCalls(post)).toHaveLength(1);
    });
    expect(downloadCalls(post)[0][1]).toEqual({ url: BINARY_URL, sha256: VALID_SHA.toUpperCase() });
  });

  it('refuses to submit a malformed checksum', async () => {
    const post = spyOnPost();
    renderCores();

    const { user, urlInput, shaInput, confirm } = await openFirstDownloadDialog();
    await user.type(urlInput, BINARY_URL);
    await user.type(shaInput, 'nope');
    await user.click(confirm);

    expect(downloadCalls(post)).toHaveLength(0);
  });

  it('refuses to submit a checksum of the wrong length', async () => {
    const post = spyOnPost();
    renderCores();

    const { user, urlInput, shaInput, confirm } = await openFirstDownloadDialog();
    await user.type(urlInput, BINARY_URL);
    await user.type(shaInput, VALID_SHA.slice(0, 63));
    await user.click(confirm);

    expect(downloadCalls(post)).toHaveLength(0);
  });

  it('keeps the url required', async () => {
    const post = spyOnPost();
    renderCores();

    const { user, shaInput, confirm } = await openFirstDownloadDialog();
    await user.type(shaInput, VALID_SHA);
    await user.click(confirm);

    expect(downloadCalls(post)).toHaveLength(0);
  });
});
