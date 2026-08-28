import { createContext, useCallback, useContext, useLayoutEffect, useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import { theme as antdTheme } from 'antd';
import type { ThemeConfig } from 'antd';

const STORAGE_DARK = 'dark-mode';
const STORAGE_ULTRA = 'isUltraDarkThemeEnabled';
// LUCX-HOOK: color palette switch (default blue / warm sand-graphite)
const STORAGE_PALETTE = 'lucx-theme-palette';

export type ThemePalette = 'default' | 'warm';
// END LUCX-HOOK

function readBool(key: string, fallback: boolean): boolean {
  const raw = localStorage.getItem(key);
  if (raw === null) return fallback;
  return raw === 'true';
}

// LUCX-HOOK: palette persistence; warm (sand/graphite) is the new default.
// First visit (no palette key yet) also defaults to light so Sand shows,
// not Graphite. Existing users already have dark-mode saved.
function readPalette(): ThemePalette {
  const raw = localStorage.getItem(STORAGE_PALETTE);
  return raw === 'default' ? 'default' : 'warm';
}
function readInitialDark(): boolean {
  const darkRaw = localStorage.getItem(STORAGE_DARK);
  if (darkRaw !== null) return darkRaw === 'true';
  if (localStorage.getItem(STORAGE_PALETTE) === null) return false;
  return true;
}
// END LUCX-HOOK

function applyDom(isDark: boolean, isUltra: boolean, palette: ThemePalette) {
  document.body.classList.remove('dark', 'light');
  document.body.classList.add(isDark ? 'dark' : 'light');
  if (isUltra) {
    document.documentElement.setAttribute('data-theme', 'ultra-dark');
  } else {
    document.documentElement.removeAttribute('data-theme');
  }
  // LUCX-HOOK: page CSS keys palette overrides off this attribute
  document.documentElement.setAttribute('data-palette', palette);
  // END LUCX-HOOK
  const msg = document.getElementById('message');
  if (msg) {
    msg.classList.remove('dark', 'light');
    msg.classList.add(isDark ? 'dark' : 'light');
  }
}

// module load so the document is in the right theme before React mounts.
const initialUltra = readBool(STORAGE_ULTRA, false);
// LUCX-HOOK: palette applied before mount, same as dark/ultra
const initialPalette = readPalette();
const initialDark = readInitialDark();
applyDom(initialDark, initialUltra, initialPalette);
// END LUCX-HOOK

const DARK_TOKENS = {
  colorBgBase: '#1a1b1f',
  colorBgLayout: '#1a1b1f',
  colorBgContainer: '#23252b',
  colorBgElevated: '#2d2f37',
};
const ULTRA_DARK_TOKENS = {
  colorBgBase: '#000',
  colorBgLayout: '#000',
  colorBgContainer: '#101013',
  colorBgElevated: '#1a1a1e',
};
const DARK_LAYOUT_TOKENS = {
  bodyBg: '#1a1b1f',
  headerBg: '#15161a',
  headerColor: '#ffffff',
  footerBg: '#1a1b1f',
  siderBg: '#15161a',
  triggerBg: '#23252b',
  triggerColor: '#ffffff',
};
const ULTRA_DARK_LAYOUT_TOKENS = {
  bodyBg: '#000',
  headerBg: '#050507',
  headerColor: '#ffffff',
  footerBg: '#000',
  siderBg: '#050507',
  triggerBg: '#1a1a1e',
  triggerColor: '#ffffff',
};
const DARK_MENU_TOKENS = {
  darkItemBg: '#15161a',
  darkSubMenuItemBg: '#1a1b1f',
  darkPopupBg: '#23252b',
};
const ULTRA_DARK_MENU_TOKENS = {
  darkItemBg: '#050507',
  darkSubMenuItemBg: '#000',
  darkPopupBg: '#101013',
};
const DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.06)',
};
const ULTRA_DARK_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.04)',
};
const STATISTIC_TOKENS = {
  contentFontSize: 17,
  titleFontSize: 11,
};
const LIGHT_CONTRAST_TOKENS = {
  colorTextDescription: 'rgba(0, 0, 0, 0.58)',
  colorTextTertiary: 'rgba(0, 0, 0, 0.58)',
  colorTextPlaceholder: '#767676',
  colorError: '#cf1322',
  colorErrorText: '#cf1322',
  colorSuccessText: '#237804',
};
const LIGHT_BUTTON_TOKENS = {
  colorPrimary: '#0958d9',
  colorPrimaryHover: '#2468e5',
  colorPrimaryActive: '#073ea8',
};

// LUCX-HOOK: warm palette (angry-box sand light / graphite dark / night ultra)
const SAND_LIGHT_TOKENS = {
  colorPrimary: '#8a6f4e',
  colorInfo: '#3f7c8c',
  colorSuccess: '#5f7d43',
  colorWarning: '#a97b20',
  colorError: '#b04a3c',
  colorBgBase: '#f6f2ea',
  colorBgLayout: '#f6f2ea',
  colorBgContainer: '#fffdf8',
  colorBgElevated: '#fffdf8',
  colorText: '#3a352c',
  colorTextSecondary: '#7b7365',
  colorTextTertiary: '#9a9284',
  colorTextDescription: 'rgba(58, 53, 44, 0.58)',
  colorTextPlaceholder: '#9a9284',
  colorBorder: '#ded6c6',
  colorBorderSecondary: '#ded6c6',
};
const SAND_LAYOUT_TOKENS = {
  bodyBg: '#f6f2ea',
  headerBg: '#f1ebdf',
  headerColor: '#3a352c',
  footerBg: '#f6f2ea',
  siderBg: '#f1ebdf',
  triggerBg: '#ece6da',
  triggerColor: '#3a352c',
};
const GRAPHITE_DARK_TOKENS = {
  colorPrimary: '#c2a882',
  colorInfo: '#83b3c4',
  colorSuccess: '#8fae6d',
  colorWarning: '#d0a95f',
  colorError: '#d4837a',
  colorBgBase: '#17181a',
  colorBgLayout: '#17181a',
  colorBgContainer: '#1e1f22',
  colorBgElevated: '#2a2c30',
  colorBorder: '#3a3d42',
  colorBorderSecondary: '#3a3d42',
};
const NIGHT_ULTRA_TOKENS = {
  colorPrimary: '#c9a97a',
  colorInfo: '#7fb4c6',
  colorSuccess: '#8bb069',
  colorWarning: '#d6a95a',
  colorError: '#d97f74',
  colorBgBase: '#0e0e10',
  colorBgLayout: '#0e0e10',
  colorBgContainer: '#141416',
  colorBgElevated: '#1d1e21',
  colorBorder: '#2b2d31',
  colorBorderSecondary: '#2b2d31',
};
const GRAPHITE_LAYOUT_TOKENS = {
  bodyBg: '#17181a',
  headerBg: '#131416',
  headerColor: '#dcdde0',
  footerBg: '#17181a',
  siderBg: '#131416',
  triggerBg: '#2a2c30',
  triggerColor: '#dcdde0',
};
const NIGHT_LAYOUT_TOKENS = {
  bodyBg: '#0e0e10',
  headerBg: '#0a0a0c',
  headerColor: '#e6e3dc',
  footerBg: '#0e0e10',
  siderBg: '#0a0a0c',
  triggerBg: '#1d1e21',
  triggerColor: '#e6e3dc',
};
const GRAPHITE_MENU_TOKENS = {
  darkItemBg: '#131416',
  darkSubMenuItemBg: '#17181a',
  darkPopupBg: '#1e1f22',
};
const NIGHT_MENU_TOKENS = {
  darkItemBg: '#0a0a0c',
  darkSubMenuItemBg: '#0e0e10',
  darkPopupBg: '#141416',
};
const GRAPHITE_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.08)',
};
const NIGHT_CARD_TOKENS = {
  colorBorderSecondary: 'rgba(255, 255, 255, 0.05)',
};
// END LUCX-HOOK

// hashed:false drops the `:where(.css-<hash>)` wrapper antd puts around every
// rule. It costs nothing in specificity — `:where()` contributes zero, so the
// panel's own `.ant-*` overrides still win — and it removes roughly 5,700
// wrappers, 16% of the generated stylesheet, from what the browser has to parse.
//
// cssVar.key pins the CSS-variable scope. Every panel page mounts its own
// ConfigProvider (there is no root one), and without a fixed key each mints a
// fresh useId-derived scope, so navigating re-serialises and re-injects the whole
// token block under a new class instead of reusing the one already in the head.
const SHARED_STYLE_CONFIG = {
  hashed: false,
  cssVar: { key: 'xui' },
} as const;

export function buildAntdThemeConfig(
  isDark: boolean,
  isUltra: boolean,
  // LUCX-HOOK: palette dimension (default blue / warm sand-graphite);
  // omitted by upstream call sites (Storybook) which stay on the blue palette
  palette: ThemePalette = 'default',
  // END LUCX-HOOK
): ThemeConfig {
  if (!isDark) {
    // LUCX-HOOK: sand light palette
    if (palette === 'warm') {
      return {
        ...SHARED_STYLE_CONFIG,
        algorithm: antdTheme.defaultAlgorithm,
        token: SAND_LIGHT_TOKENS,
        components: {
          Statistic: STATISTIC_TOKENS,
          Layout: SAND_LAYOUT_TOKENS,
        },
      };
    }
    // END LUCX-HOOK
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.defaultAlgorithm,
      token: LIGHT_CONTRAST_TOKENS,
      components: {
        Statistic: STATISTIC_TOKENS,
        Button: LIGHT_BUTTON_TOKENS,
      },
    };
  }
  // LUCX-HOOK: graphite dark / night ultra palettes
  if (palette === 'warm') {
    return {
      ...SHARED_STYLE_CONFIG,
      algorithm: antdTheme.darkAlgorithm,
      token: isUltra ? NIGHT_ULTRA_TOKENS : GRAPHITE_DARK_TOKENS,
      components: {
        Layout: isUltra ? NIGHT_LAYOUT_TOKENS : GRAPHITE_LAYOUT_TOKENS,
        Menu: isUltra ? NIGHT_MENU_TOKENS : GRAPHITE_MENU_TOKENS,
        Card: isUltra ? NIGHT_CARD_TOKENS : GRAPHITE_CARD_TOKENS,
        Statistic: STATISTIC_TOKENS,
      },
    };
  }
  // END LUCX-HOOK
  return {
    ...SHARED_STYLE_CONFIG,
    algorithm: antdTheme.darkAlgorithm,
    token: isUltra ? ULTRA_DARK_TOKENS : DARK_TOKENS,
    components: {
      Layout: isUltra ? ULTRA_DARK_LAYOUT_TOKENS : DARK_LAYOUT_TOKENS,
      Menu: isUltra ? ULTRA_DARK_MENU_TOKENS : DARK_MENU_TOKENS,
      Card: isUltra ? ULTRA_DARK_CARD_TOKENS : DARK_CARD_TOKENS,
      Statistic: STATISTIC_TOKENS,
    },
  };
}

export function pauseAnimationsUntilLeave(elementId: string): void {
  document.documentElement.setAttribute('data-theme-animations', 'off');
  const el = document.getElementById(elementId);
  if (!el) return;
  const restore = () => {
    document.documentElement.removeAttribute('data-theme-animations');
    el.removeEventListener('mouseleave', restore);
    el.removeEventListener('touchend', restore);
  };
  el.addEventListener('mouseleave', restore);
  el.addEventListener('touchend', restore);
}

interface ThemeContextValue {
  isDark: boolean;
  isUltra: boolean;
  toggleTheme: () => void;
  toggleUltra: () => void;
  // LUCX-HOOK: color palette (default blue / warm sand-graphite)
  palette: ThemePalette;
  togglePalette: () => void;
  // END LUCX-HOOK
  antdThemeConfig: ThemeConfig;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

export function ThemeProvider({ children }: { children: ReactNode }) {
  const [isDark, setIsDark] = useState<boolean>(initialDark);
  const [isUltra, setIsUltra] = useState<boolean>(initialUltra);
  // LUCX-HOOK: palette state
  const [palette, setPalette] = useState<ThemePalette>(initialPalette);
  // END LUCX-HOOK

  useLayoutEffect(() => {
    applyDom(isDark, isUltra, palette);
    localStorage.setItem(STORAGE_DARK, String(isDark));
    localStorage.setItem(STORAGE_ULTRA, String(isUltra));
    // LUCX-HOOK: persist palette
    localStorage.setItem(STORAGE_PALETTE, palette);
    // END LUCX-HOOK
  }, [isDark, isUltra, palette]);

  const toggleTheme = useCallback(() => setIsDark((v) => !v), []);
  const toggleUltra = useCallback(() => setIsUltra((v) => !v), []);
  // LUCX-HOOK: cycle between the two palettes
  const togglePalette = useCallback(
    () => setPalette((p) => (p === 'default' ? 'warm' : 'default')),
    [],
  );
  // END LUCX-HOOK

  const antdThemeConfig = useMemo(
    () => buildAntdThemeConfig(isDark, isUltra, palette),
    [isDark, isUltra, palette],
  );

  const value = useMemo<ThemeContextValue>(
    () => ({ isDark, isUltra, toggleTheme, toggleUltra, palette, togglePalette, antdThemeConfig }),
    [isDark, isUltra, toggleTheme, toggleUltra, palette, togglePalette, antdThemeConfig],
  );

  return <ThemeContext.Provider value={value}>{children}</ThemeContext.Provider>;
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext);
  if (!ctx) throw new Error('useTheme must be used inside <ThemeProvider>');
  return ctx;
}
