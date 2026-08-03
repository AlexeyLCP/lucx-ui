import { useLayoutEffect } from 'react';
import type { Decorator, Preview } from '@storybook/react-vite';
import { ConfigProvider } from 'antd';
import i18next from 'i18next';
import { initReactI18next } from 'react-i18next';

import { buildAntdThemeConfig } from '@/hooks/useTheme';
import enUS from '../../internal/web/translation/en-US.json';

if (!i18next.isInitialized) {
  void i18next.use(initReactI18next).init({
    lng: 'en-US',
    fallbackLng: 'en-US',
    resources: { 'en-US': { translation: enUS } },
    interpolation: { escapeValue: false, prefix: '{', suffix: '}' },
    returnNull: false,
  });
}

export const withTheme: Decorator = (Story, context) => {
  const dark = context.globals.theme === 'dark';
  useLayoutEffect(() => {
    document.body.classList.remove('dark', 'light');
    document.body.classList.add(dark ? 'dark' : 'light');
    document.documentElement.removeAttribute('data-theme');
  }, [dark]);
  // token.motion:false makes antd expand/collapse instant inside stories. The
  // a11y addon runs axe right after play() resolves; with motion on, Collapse's
  // content is still fading in and axe samples the text mid-animation at a
  // partial opacity, computing a blended low-contrast colour (ConfigBlock
  // "Collapsed" flaked at #a6a6a6 on #f8f8f8, 2.29:1). Deterministic render
  // removes the race without touching the production app's animations.
  const themeConfig = buildAntdThemeConfig(dark, false);
  return (
    <ConfigProvider theme={{ ...themeConfig, token: { ...themeConfig.token, motion: false } }}>
      <div style={{ padding: 24, minWidth: 320 }}>
        <Story />
      </div>
    </ConfigProvider>
  );
};

const preview: Preview = {
  decorators: [withTheme],
  globalTypes: {
    theme: {
      description: 'Ant Design theme',
      defaultValue: 'light',
      toolbar: {
        title: 'Theme',
        icon: 'circlehollow',
        items: [
          { value: 'light', title: 'Light' },
          { value: 'dark', title: 'Dark' },
        ],
        dynamicTitle: true,
      },
    },
  },
  parameters: {
    controls: {
      expanded: true,
      sort: 'requiredFirst',
      matchers: {
        color: /(background|color)$/i,
        date: /Date$/i,
      },
    },
    a11y: {
      test: 'error',
    },
  },
};

export default preview;
