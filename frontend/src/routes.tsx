import { Suspense } from 'react';
import { createBrowserRouter, useRouteError, type RouteObject } from 'react-router';
import { Spin } from 'antd';

import PanelLayout from '@/layouts/PanelLayout';
import { reloadOnceOnStaleChunk } from '@/lib/stale-chunk'; // LUCX-HOOK: stale chunk reload
import { PANEL_ROUTES } from '@/routes/panelRoutes';

function withSuspense(node: React.ReactNode) {
  return (
    <Suspense
      fallback={
        <div
          style={{
            display: 'flex',
            justifyContent: 'center',
            alignItems: 'center',
            minHeight: '60vh',
          }}
        >
          <Spin size="large" />
        </div>
      }
    >
      {node}
    </Suspense>
  );
}

// LUCX-HOOK: a deploy replaces the hashed chunks; an already-open tab that
// lazy-loads a page afterwards gets a chunk fetch failure. Reload once, then
// surface the error.
function StaleAssetReload() {
  const error = useRouteError();
  if (reloadOnceOnStaleChunk(error)) return null;
  const msg = error instanceof Error ? error.message : String(error ?? '');
  return (
    <div style={{ padding: 48, textAlign: 'center' }}>{msg || 'Unexpected Application Error'}</div>
  );
}

export const routes: RouteObject[] = [
  {
    path: '/',
    element: <PanelLayout />,
    errorElement: <StaleAssetReload />,
    children: PANEL_ROUTES.map((route) => {
      const element = withSuspense(<route.component />);
      return route.path === '/' ? { index: true, element } : { path: route.path.slice(1), element };
    }),
  },
];

function computeBasename() {
  const raw = (typeof window !== 'undefined' && window.X_UI_BASE_PATH) || '/';
  const trimmed = raw.replace(/\/+$/, '');
  return `${trimmed}/panel`;
}

export const router = createBrowserRouter(routes, {
  basename: computeBasename(),
});
