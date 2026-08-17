import { Fragment } from 'react';
import type { ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Tag, Tooltip } from 'antd';
import {
  ArrowUpOutlined,
  AreaChartOutlined,
  BarsOutlined,
  CloudDownloadOutlined,
  CloudServerOutlined,
  ControlOutlined,
  FileTextOutlined,
  PoweroffOutlined,
  ReloadOutlined,
} from '@ant-design/icons';

import { formatPanelVersion } from '@/lib/panel-version';
import type { Status } from '@/models/status';

interface OverviewActionBarProps {
  status: Status;
  isMobile: boolean;
  accessLogEnable: boolean;
  panelVersion: string;
  latestVersion: string;
  updateAvailable: boolean;
  onStopXray: () => void;
  onRestartXray: () => void;
  onRestartAwg: () => void;
  onOpenLogs: () => void;
  onOpenXrayLogs: () => void;
  onOpenConfig: () => void;
  onOpenBackup: () => void;
  onOpenSystemHistory: () => void;
  onOpenXrayMetrics: () => void;
  onOpenPanelUpdate: () => void;
  onOpenVersionSwitch: () => void;
}

interface BarAction {
  key: string;
  icon: ReactNode;
  text: string;
  onClick: () => void;
  primary?: boolean;
}

const XRAY_STATE_KEYS: Record<string, string> = {
  running: 'pages.index.xrayStatusRunning',
  stop: 'pages.index.xrayStatusStop',
  error: 'pages.index.xrayStatusError',
};

const AWG_STATE_KEYS: Record<string, string> = {
  running: 'pages.index.awgStatusRunning',
  stop: 'pages.index.awgStatusStop',
  error: 'pages.index.awgStatusError',
};

export default function OverviewActionBar({
  status,
  isMobile,
  accessLogEnable,
  panelVersion,
  latestVersion,
  updateAvailable,
  onStopXray,
  onRestartXray,
  onRestartAwg,
  onOpenLogs,
  onOpenXrayLogs,
  onOpenConfig,
  onOpenBackup,
  onOpenSystemHistory,
  onOpenXrayMetrics,
  onOpenPanelUpdate,
  onOpenVersionSwitch,
}: OverviewActionBarProps) {
  const { t } = useTranslation();
  const stateText = t(XRAY_STATE_KEYS[status.xray.state] ?? 'pages.index.xrayStatusUnknown');
  const hasVersion = !!status.xray.version && status.xray.version !== 'Unknown';
  const awgStateText = t(AWG_STATE_KEYS[status.awg.state] ?? 'pages.index.awgStatusUnknown');
  const awgVersion = (status.awg.version || '').replace(/^v/i, '');
  const hasAwgVersion = !!awgVersion;
  const size = isMobile ? ('small' as const) : ('middle' as const);

  const actionGroups: BarAction[][] = [
    [
      { key: 'restart', icon: <ReloadOutlined />, text: t('pages.index.restartXray'), onClick: onRestartXray, primary: true },
      { key: 'stop', icon: <PoweroffOutlined />, text: t('pages.index.stopXray'), onClick: onStopXray },
      { key: 'awgRestart', icon: <ReloadOutlined />, text: t('pages.index.awgRestart'), onClick: onRestartAwg },
    ],
    [
      { key: 'logs', icon: <BarsOutlined />, text: t('pages.index.logs'), onClick: onOpenLogs },
      ...(accessLogEnable
        ? [{ key: 'accessLogs', icon: <FileTextOutlined />, text: t('pages.index.accessLogs'), onClick: onOpenXrayLogs }]
        : []),
      { key: 'config', icon: <ControlOutlined />, text: t('pages.index.config'), onClick: onOpenConfig },
      { key: 'backup', icon: <CloudServerOutlined />, text: t('pages.index.backupTitle'), onClick: onOpenBackup },
    ],
    [
      { key: 'history', icon: <AreaChartOutlined />, text: t('pages.index.systemHistoryTitle'), onClick: onOpenSystemHistory },
      { key: 'metrics', icon: <ArrowUpOutlined />, text: t('pages.index.xrayMetricsTitle'), onClick: onOpenXrayMetrics },
    ],
  ];

  const statePill = (
    <span className="ov-state" data-state={status.xray.state}>
      <span className="ov-state-dot" style={{ color: status.xray.color }} />
      <span>{`${t('pages.index.xrayStatus')} · ${stateText}`}</span>
      {hasVersion && (
        <Tooltip title={t('pages.index.xraySwitch')}>
          <button
            type="button"
            className="ov-state-version"
            onClick={onOpenVersionSwitch}
          >
            {`v${status.xray.version}`}
          </button>
        </Tooltip>
      )}
    </span>
  );

  const ifList = (status.awg.ifnames || []).join(', ');
  const awgModuleMissing = !status.awg.moduleLoaded;
  const awgErrorText =
    status.awg.errorMsg === 'module_not_loaded' || awgModuleMissing
      ? t('pages.index.awgModuleNotLoadedHint')
      : status.awg.errorMsg || '';
  const awgTipParts = [
    awgErrorText,
    status.awg.moduleLoaded
      ? t('pages.index.awgInterfaces', { n: status.awg.interfaces })
      : t('pages.index.awgModuleNotLoaded'),
    ifList ? ifList : '',
    status.awg.moduleAwg31 ? 'AWG3.1 OK' : status.awg.moduleAwg3 ? 'AWG3 OK' : '',
  ].filter(Boolean);
  const awgTip = awgTipParts.join('\n');

  const awgPill = (
    <span className="ov-state" data-state={status.awg.state}>
      <span className="ov-state-dot" style={{ color: status.awg.color }} />
      <span>{`${t('pages.index.awgStatus')} · ${awgStateText}`}</span>
      {hasAwgVersion && (
        <span className="ov-state-version" style={{ cursor: 'default' }}>
          {`v${awgVersion}`}
        </span>
      )}
    </span>
  );

  return (
    <div className="ov-bar">
      {status.xray.state === 'error' && status.xray.errorMsg ? (
        <Tooltip title={<span className="ov-error-detail">{status.xray.errorMsg}</span>}>
          {statePill}
        </Tooltip>
      ) : (
        statePill
      )}

      {awgTip ? (
        <Tooltip title={<span className="ov-error-detail" style={{ whiteSpace: 'pre-line' }}>{awgTip}</span>}>
          {awgPill}
        </Tooltip>
      ) : (
        awgPill
      )}
      {awgModuleMissing && (
        <Tag color="error" style={{ marginInlineStart: 4, maxWidth: 420, whiteSpace: 'normal', height: 'auto' }}>
          {t('pages.index.awgModuleNotLoadedHint')}
        </Tag>
      )}

      {updateAvailable ? (
        <Tag
          className="ov-update-tag"
          color="warning"
          icon={<CloudDownloadOutlined />}
          onClick={onOpenPanelUpdate}
        >
          {`${t('update')} ${formatPanelVersion(latestVersion)}`}
        </Tag>
      ) : (
        <Tooltip title={t('pages.index.updatePanel')}>
          <button type="button" className="ov-panel-version ov-mono" onClick={onOpenPanelUpdate}>
            {formatPanelVersion(panelVersion)}
          </button>
        </Tooltip>
      )}

      <div className="ov-bar-actions">
        {actionGroups.map((group, groupIndex) => (
          <Fragment key={group[0].key}>
            {groupIndex > 0 && <span className="ov-bar-sep" />}
            {group.map((action) => (
              <Button
                key={action.key}
                type={action.primary ? undefined : 'text'}
                color={action.primary ? 'primary' : undefined}
                variant={action.primary ? 'outlined' : undefined}
                size={size}
                icon={action.icon}
                aria-label={action.text}
                onClick={action.onClick}
              >
                {isMobile ? undefined : action.text}
              </Button>
            ))}
          </Fragment>
        ))}
      </div>
    </div>
  );
}
