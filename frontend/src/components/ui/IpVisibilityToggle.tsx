import { EyeOutlined, EyeInvisibleOutlined } from '@ant-design/icons';
import { Tooltip } from 'antd';
import { useTranslation } from 'react-i18next';

import { activateOnKey } from '@/utils/a11y';

/*
 * The eye that toggles blurred IP reveal (dashboard strip, nodes, outbounds).
 * The two icon glyphs swap inside one stable wrapper so the Tooltip's trigger
 * element never changes identity: swapping the tooltip's child directly
 * re-anchors rc-trigger on hover and flaps it (layout-jump report, 24.11).
 */
export function IpVisibilityToggle({
  visible,
  onToggle,
}: {
  visible: boolean;
  onToggle: () => void;
}) {
  const { t } = useTranslation();
  const label = t('pages.index.toggleIpVisibility');
  return (
    <Tooltip title={label} placement="top" mouseLeaveDelay={0.2}>
      <span
        className="ip-toggle-wrap"
        role="button"
        tabIndex={0}
        aria-label={label}
        onClick={onToggle}
        onKeyDown={activateOnKey(onToggle)}
      >
        {visible ? (
          <EyeOutlined className="ip-toggle-icon" aria-hidden />
        ) : (
          <EyeInvisibleOutlined className="ip-toggle-icon" aria-hidden />
        )}
      </span>
    </Tooltip>
  );
}
