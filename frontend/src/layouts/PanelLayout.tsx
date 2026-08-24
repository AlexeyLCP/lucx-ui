import { Outlet } from 'react-router';

import { useWebSocketBridge } from '@/api/websocketBridge';
import { usePageTitle } from '@/hooks/usePageTitle';
import { usePanelFavicon } from '@/hooks/usePanelFavicon';

export default function PanelLayout() {
  useWebSocketBridge();
  usePageTitle();
  usePanelFavicon();
  return <Outlet />;
}
