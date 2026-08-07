import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Tag, Typography } from 'antd';
import { CloudDownloadOutlined } from '@ant-design/icons';

import { HttpUtil, PromiseUtil } from '@/utils';
import { formatPanelVersion } from '@/lib/panel-version';
import type { PanelUpdateStatus } from '@/generated/types';
import './PanelUpdateModal.css';

type UpdateOutcome = 'success' | 'failed' | 'timeout';

export interface PanelUpdateInfo {
  channel?: string;
  currentVersion: string;
  latestVersion: string;
  currentCommit?: string;
  latestCommit?: string;
  updateAvailable: boolean;
  releaseNotes?: string;
}

interface BusyEvent {
  busy: boolean;
  tip?: string;
}

interface PanelUpdateModalProps {
  open: boolean;
  info: PanelUpdateInfo;
  onClose: () => void;
  onBusy: (e: BusyEvent) => void;
}

const POLL_INITIAL_MS = 5_000;
const POLL_DEADLINE_MS = 15 * 60_000;
const POLL_INTERVAL_MS = 3_000;

export default function PanelUpdateModal({
  open,
  info,
  onClose,
  onBusy,
}: PanelUpdateModalProps) {
  const { t } = useTranslation();
  const [modal, contextHolder] = Modal.useModal();
  const [notesExpanded, setNotesExpanded] = useState(false);

  const notes = (info.releaseNotes || '').trim();

  async function pollUpdateStatus(expectedRunId: string): Promise<UpdateOutcome> {
    await PromiseUtil.sleep(POLL_INITIAL_MS);
    const deadline = Date.now() + POLL_DEADLINE_MS;
    while (Date.now() < deadline) {
      try {
        const msg = await HttpUtil.get<PanelUpdateStatus>(
          '/panel/api/server/getUpdateStatus',
          undefined,
          { silent: true, timeout: 2000 },
        );
        const status = msg?.obj ?? undefined;
        if (status?.runId === expectedRunId) {
          if (status.state === 'success') return 'success';
          if (status.state === 'failed') return 'failed';
        }
      } catch {
        /* still restarting */
      }
      await PromiseUtil.sleep(POLL_INTERVAL_MS);
    }
    return 'timeout';
  }

  function updatePanel() {
    modal.confirm({
      title: t('pages.index.panelUpdateDialog'),
      content: t('pages.index.panelUpdateDialogDesc').replace('#version#', info.latestVersion || ''),
      okText: t('confirm'),
      cancelText: t('cancel'),
      onOk: async () => {
        const baseTip = t('pages.index.dontRefresh');
        const tip = info.latestVersion ? `${baseTip} (${info.latestVersion})` : baseTip;
        onClose();
        onBusy({ busy: true, tip });
        const result = await HttpUtil.post<{ runId: string }>('/panel/api/server/updatePanel');
        if (!result?.success) {
          onBusy({ busy: false });
          return;
        }
        const outcome = await pollUpdateStatus(result.obj?.runId ?? '');
        onBusy({ busy: false });
        if (outcome === 'success') {
          await PromiseUtil.sleep(800);
          window.location.reload();
          return;
        }
        modal[outcome === 'failed' ? 'error' : 'warning']({
          title: t(outcome === 'failed' ? 'pages.index.panelUpdateFailedTitle' : 'pages.index.panelUpdateUnknownTitle'),
          content: t(outcome === 'failed' ? 'pages.index.panelUpdateFailedDesc' : 'pages.index.panelUpdateUnknownDesc'),
          okText: t('refresh'),
          onOk: () => window.location.reload(),
        });
      },
    });
  }

  return (
    <>
      {contextHolder}
      <Modal
        open={open}
        title={t('pages.index.updatePanel')}
        footer={null}
        onCancel={onClose}
        width={560}
      >
        {info.updateAvailable && (
          <Alert
            type="warning"
            className="mb-12"
            title={t('pages.index.panelUpdateDesc')}
            showIcon
          />
        )}

        <div className="version-list">
          <div className="version-list-item">
            <span>{t('pages.index.currentPanelVersion')}</span>
            <Tag color="green">{formatPanelVersion(window.X_UI_CUR_VER || info.currentVersion) || '?'}</Tag>
          </div>
          {info.updateAvailable ? (
            <div className="version-list-item">
              <span>{t('pages.index.latestPanelVersion')}</span>
              <Tag color="purple">{info.latestVersion || '-'}</Tag>
            </div>
          ) : (
            <div className="version-list-item">
              <span>{t('pages.index.panelUpToDate')}</span>
              <Tag color="green">{t('pages.index.panelUpToDate')}</Tag>
            </div>
          )}
        </div>

        {info.updateAvailable && notes && (
          <div className="release-notes">
            <div className="release-notes-title">{t('pages.index.releaseNotes')}</div>
            <Typography.Paragraph
              className="release-notes-body"
              ellipsis={
                notesExpanded
                  ? false
                  : { rows: 8, expandable: true, symbol: t('pages.index.releaseNotesMore'), onExpand: () => setNotesExpanded(true) }
              }
            >
              {notes}
            </Typography.Paragraph>
          </div>
        )}

        <div className="actions-row">
          <Button
            type="primary"
            disabled={!info.updateAvailable}
            onClick={updatePanel}
            icon={<CloudDownloadOutlined />}
          >
            {t('pages.index.updatePanel')}
          </Button>
        </div>
      </Modal>
    </>
  );
}
