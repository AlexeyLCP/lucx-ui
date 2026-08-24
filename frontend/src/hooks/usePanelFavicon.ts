import { useEffect } from 'react';

import { useAllSettings } from '@/api/queries/useAllSettings';
import { applyPanelFavicon } from '@/lib/panelFavicon';

export function usePanelFavicon() {
  const { allSetting } = useAllSettings();

  useEffect(() => {
    applyPanelFavicon(allSetting.webFavicon ?? '');
  }, [allSetting.webFavicon]);
}
