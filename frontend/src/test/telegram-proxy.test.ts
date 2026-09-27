import { describe, expect, it } from 'vitest';

import { AllSetting } from '@/models/setting';

describe('Telegram proxy settings', () => {
  it('retains the server proxy when an unrelated setting is edited and serialized', () => {
    const settings = new AllSetting({
      tgBotProxy: 'socks5://127.0.0.1:10881',
      tgBotEnable: true,
    });
    settings.tgLang = 'ru-RU';
    expect(JSON.parse(JSON.stringify(settings)).tgBotProxy).toBe('socks5://127.0.0.1:10881');
  });

  it('allows explicitly clearing the proxy', () => {
    const settings = new AllSetting({ tgBotProxy: 'socks5://127.0.0.1:10881' });
    settings.tgBotProxy = '';
    expect(JSON.parse(JSON.stringify(settings)).tgBotProxy).toBe('');
  });
});
