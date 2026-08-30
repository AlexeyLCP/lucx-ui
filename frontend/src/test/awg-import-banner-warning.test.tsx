import { beforeEach, describe, expect, it, vi } from 'vitest';
import { fireEvent, screen, waitFor } from '@testing-library/react';

import AwgImportBanner from '@/pages/inbounds/AwgImportBanner';
import { awgImportApi } from '@/api/awg-import';
import { Msg } from '@/utils';
import type { AwgImportCandidate, AwgImportResult } from '@/schemas/awg-import';

import { renderWithProviders } from './test-utils';

const { messageSuccess, messageWarning, messageError } = vi.hoisted(() => ({
  messageSuccess: vi.fn(),
  messageWarning: vi.fn(),
  messageError: vi.fn(),
}));

vi.mock('antd', async (importOriginal) => {
  const actual = await importOriginal<typeof import('antd')>();
  return {
    ...actual,
    message: {
      ...actual.message,
      success: messageSuccess,
      warning: messageWarning,
      error: messageError,
    },
  };
});

vi.mock('@/api/awg-import', () => ({
  awgImportApi: { preview: vi.fn(), dismiss: vi.fn(), commit: vi.fn() },
}));

const IFIELD_WARNING =
  'saved, I-fields will not be applied: awg: I1-I5 exceed the netlink read budget: 3632 > 3492 worst-case bytes for awgo-1';

function candidate(): AwgImportCandidate {
  return {
    id: 'amnezia:awg0',
    source: 'amnezia',
    ifname: 'awg0',
    confPath: '/etc/amnezia/amneziawg/awg0.conf',
    live: true,
    port: 51820,
    address: '10.8.1.1/24',
    awgVersion: '1.5',
    peerCount: 3,
    namedPeers: 3,
    keysFound: 3,
    handshakes: 2,
    suspended: 0,
    backend: 'kernel',
    dropOnImport: false,
    warning: '',
    stopTarget: '',
    peers: [],
  };
}

function result(over: Partial<AwgImportResult>): AwgImportResult {
  return {
    id: 'amnezia:awg0',
    inboundId: 12,
    remark: 'awg0',
    clients: 3,
    missingKeys: 0,
    adopted: true,
    stopped: true,
    ...over,
  };
}

// Whether the source is still discoverable after the commit decides what the
// modal can show, so each case states it rather than sharing one default.
async function importOnce(results: AwgImportResult[], stillDiscoverable: boolean) {
  const preview = vi.mocked(awgImportApi.preview);
  preview.mockResolvedValue(new Msg(true, '', { dismissed: false, candidates: [candidate()] }));
  vi.mocked(awgImportApi.commit).mockResolvedValue(new Msg(true, '', results));
  const onImported = vi.fn();
  renderWithProviders(<AwgImportBanner openMenu={1} onImported={onImported} />);
  await screen.findByText('awg0');
  if (!stillDiscoverable) {
    preview.mockResolvedValue(new Msg(true, '', { dismissed: false, candidates: [] }));
  }
  fireEvent.click(screen.getByRole('button', { name: 'Import selected' }));
  await waitFor(() => expect(awgImportApi.commit).toHaveBeenCalledTimes(1));
  return onImported;
}

const errorAlerts = () =>
  Array.from(document.querySelectorAll('.ant-alert-error')).map((el) =>
    (el.textContent ?? '').trim(),
  );

beforeEach(() => {
  messageSuccess.mockClear();
  messageWarning.mockClear();
  messageError.mockClear();
  vi.mocked(awgImportApi.preview).mockClear();
  vi.mocked(awgImportApi.commit).mockClear();
});

describe('AwgImportBanner import outcomes', () => {
  it('counts a saved-with-warning import as a success', async () => {
    const onImported = await importOnce([result({ warning: IFIELD_WARNING })], false);

    await waitFor(() => expect(onImported).toHaveBeenCalledTimes(1));
    expect(messageSuccess).toHaveBeenCalledTimes(1);
    expect(messageWarning.mock.calls.flat().join(' ')).toContain(IFIELD_WARNING);
    expect(errorAlerts()).toEqual([]);
    await waitFor(() => expect(document.querySelector('.ant-modal')).toBeNull());
  });

  it('still reports a real failure in red and keeps the modal open', async () => {
    const onImported = await importOnce([result({ error: 'backup failed: no space left' })], true);

    await waitFor(() => expect(errorAlerts().join(' ')).toContain('backup failed: no space left'));
    expect(onImported).not.toHaveBeenCalled();
    expect(messageSuccess).not.toHaveBeenCalled();
    expect(document.querySelector('.ant-modal')).not.toBeNull();
  });

  it('delivers both channels when a saved import also hit a later failure', async () => {
    await importOnce(
      [result({ error: 'saved, stop old source failed: boom', warning: IFIELD_WARNING })],
      true,
    );

    await waitFor(() =>
      expect(errorAlerts().join(' ')).toContain('saved, stop old source failed: boom'),
    );
    expect(messageWarning.mock.calls.flat().join(' ')).toContain(IFIELD_WARNING);
  });
});
