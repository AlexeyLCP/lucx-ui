import { screen, within } from '@testing-library/react';
import { MemoryRouter } from 'react-router';
import { beforeAll, describe, expect, it, vi } from 'vitest';

import PluginManagerPage from '@/pages/plugins/PluginManagerPage';
import type { PluginStatus } from '@/api/plugins';
import { renderWithProviders } from './test-utils';
import { HttpUtil, Msg } from '@/utils';

vi.mock('@/api/queries/useAllSettings', () => ({
  useAllSettings: () => ({ allSetting: {} }),
}));

const plugins: PluginStatus[] = [
  {
    manifest: {
      id: 'demo',
      name: 'Demo',
      version: '1.0.0',
      type: 'sidecar',
      permissions: { network: ['none'], panelApi: [], bot: [], sub: ['after'] },
      resources: { maxMemoryMB: 64, maxCPUPercent: 25 },
      events: ['cpu.high', 'xray.crash'],
      health: { type: 'http', target: 'http://127.0.0.1:8080/healthz' },
    },
    installed: true,
    running: true,
    runner: 'unit',
    binaryPath: 'bin/plugins/demo/run.sh',
    dataPath: 'bin/plugins/demo/data',
    logPath: 'bin/plugins/demo/plugin.log',
    enforcement: {
      networkIsolated: true,
      memoryLimitMB: 64,
      enforceable: true,
      cpuLimitPercent: 25,
      cpuEnforced: true,
    },
    health: { status: 'fail', detail: 'connect refused', checkedAt: 1 },
  },
  {
    manifest: { id: 'plain', name: 'Plain', version: '0.1.0', type: 'sidecar' },
    installed: true,
    running: false,
    binaryPath: 'bin/plugins/plain/run.sh',
    dataPath: 'bin/plugins/plain/data',
    logPath: 'bin/plugins/plain/plugin.log',
    enforcement: { networkIsolated: false, enforceable: true },
  },
  {
    manifest: {
      id: 'asked',
      name: 'Ask enforce',
      version: '0.1.0',
      type: 'sidecar',
      permissions: { network: ['none'], panelApi: [], bot: [] },
    },
    installed: true,
    running: false,
    binaryPath: 'bin/plugins/asked/run.sh',
    dataPath: 'bin/plugins/asked/data',
    logPath: 'bin/plugins/asked/plugin.log',
    enforcement: { networkIsolated: true, enforceable: false },
  },
];

beforeAll(async () => {
  const { readyI18n } = await import('@/i18n/react');
  await readyI18n('panel');
});

function renderPlugins() {
  return renderWithProviders(
    <MemoryRouter>
      <PluginManagerPage />
    </MemoryRouter>,
  );
}

// Card titles and copyable paragraphs split text across nodes, so pick the
// card by its full textContent instead of getByText.
function cardWithText(fragment: string): HTMLElement {
  const card = [...document.querySelectorAll('.ant-card')].find((el) =>
    (el.textContent ?? '').includes(fragment),
  );
  if (!card) throw new Error(`plugin card with ${fragment} not found`);
  return card as HTMLElement;
}

describe('PluginManagerPage', () => {
  it('renders Stage 2 enforcement and health from the plugin status', async () => {
    vi.spyOn(HttpUtil, 'get').mockImplementation(async (url: string) =>
      String(url).includes('/plugins/list') ? new Msg(true, '', plugins) : new Msg(true, '', null),
    );
    renderPlugins();
    await screen.findByText('Demo');
    const card = cardWithText('bin/plugins/demo/run.sh');
    await within(card).findByText('Unhealthy');
    expect(within(card).getByText('Network isolated')).toBeTruthy();
    expect(within(card).getByText('Memory limit: 64 MB')).toBeTruthy();
    expect(within(card).getByText('CPU limit: 25%')).toBeTruthy();
    expect(within(card).getByText('systemd unit')).toBeTruthy();
    expect(within(card).getByText('Events subscribed: 2')).toBeTruthy();
    expect(within(card).getByText('Injects sub links (sub/after)')).toBeTruthy();
    const askedCard = cardWithText('bin/plugins/asked/run.sh');
    expect(within(askedCard).getByText('Not enforced on this host')).toBeTruthy();
    expect(screen.getAllByText('Stopped').length).toBeGreaterThan(0);
  });

  it('stays silent when a plugin requests no enforcement or probe', async () => {
    vi.spyOn(HttpUtil, 'get').mockImplementation(async () => new Msg(true, '', [plugins[1]]));
    renderPlugins();
    await screen.findByText('Plain');
    expect(screen.queryByText('Network isolated')).toBeNull();
    expect(screen.queryByText('Unhealthy')).toBeNull();
  });
});
