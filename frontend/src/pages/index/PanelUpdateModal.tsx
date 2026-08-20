import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Spin, Tag, Typography } from 'antd';
import { CloudDownloadOutlined } from '@ant-design/icons';

import { HttpUtil, PromiseUtil } from '@/utils';
import { formatPanelVersion } from '@/lib/panel-version';
import type { PanelReleaseNote, PanelReleaseNotes, PanelUpdateStatus } from '@/generated/types';
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
  const [feed, setFeed] = useState<PanelReleaseNote[]>([]);
  const [hasMore, setHasMore] = useState(false);
  const [page, setPage] = useState(1);
  const [feedLoading, setFeedLoading] = useState(false);
  const [moreLoading, setMoreLoading] = useState(false);

  const notes = (info.releaseNotes || '').trim();

  useEffect(() => {
    if (!open || !info.updateAvailable) {
      setFeed([]);
      setHasMore(false);
      setPage(1);
      setFeedLoading(false);
      setMoreLoading(false);
      setNotesExpanded(false);
      return;
    }
    let cancelled = false;
    setFeedLoading(true);
    HttpUtil.get<PanelReleaseNotes>(
      '/panel/api/server/getPanelReleaseNotes',
      { page: 1 },
      { silent: true },
    )
      .then((msg) => {
        if (cancelled) return;
        const obj = msg?.success ? msg.obj : null;
        setFeed(obj?.items ?? []);
        setHasMore(!!obj?.hasMore);
        setPage(obj?.page ?? 1);
      })
      .finally(() => {
        if (!cancelled) setFeedLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, [open, info.updateAvailable]);

  async function loadMoreNotes() {
    const next = page + 1;
    setMoreLoading(true);
    try {
      const msg = await HttpUtil.get<PanelReleaseNotes>(
        '/panel/api/server/getPanelReleaseNotes',
        { page: next },
        { silent: true },
      );
      const obj = msg?.success ? msg.obj : null;
      if (!obj) {
        setHasMore(false);
        return;
      }
      setFeed((prev) => [...prev, ...(obj.items ?? [])]);
      setHasMore(!!obj.hasMore);
      setPage(obj.page ?? next);
    } finally {
      setMoreLoading(false);
    }
  }

  function formatPublished(iso?: string): string {
    if (!iso) return '';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return iso;
    return d.toLocaleDateString();
  }

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
        width={640}
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

        {info.updateAvailable && (feedLoading || feed.length > 0 || notes) && (
          <div className="release-notes">
            <div className="release-notes-title">{t('pages.index.releaseNotes')}</div>
            {feedLoading ? (
              <div className="release-notes-loading">
                <Spin size="small" />
              </div>
            ) : feed.length > 0 ? (
              <>
                {feed.map((item) => (
                  <div key={item.tag} className="release-note-item">
                    <div className="release-note-meta">
                      <Tag color="purple">{item.tag}</Tag>
                      {item.publishedAt ? (
                        <span className="release-note-date">{formatPublished(item.publishedAt)}</span>
                      ) : null}
                    </div>
                    {item.body ? (
                      <Typography.Paragraph className="release-notes-body">
                        {item.body}
                      </Typography.Paragraph>
                    ) : null}
                  </div>
                ))}
                {hasMore ? (
                  <Button
                    type="link"
                    size="small"
                    loading={moreLoading}
                    onClick={() => {
                      void loadMoreNotes();
                    }}
                  >
                    {t('pages.index.releaseNotesLoadMore')}
                  </Button>
                ) : null}
              </>
            ) : (
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
            )}
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
