import { useCallback, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from 'react-router';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Badge,
  Button,
  Card,
  Divider,
  Input,
  Modal,
  Popconfirm,
  Space,
  Tag,
  Typography,
  Upload,
} from 'antd';
import {
  CloudDownloadOutlined,
  DeleteOutlined,
  FileSearchOutlined,
  ReloadOutlined,
  UploadOutlined,
} from '@ant-design/icons';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import { tunnelsApi } from '@/api/tunnels';
import type { AwgInfo } from '@/models/status';

type CoreKind = 'naive' | 'olcrtc' | 'qwdtt' | 'mieru' | 'trusttunnel';

interface CoreStatusView {
  probe?: { running?: boolean };
  binaryExists?: boolean;
  binaryPath?: string;
}

interface CoreApiResult<T> {
  success?: boolean;
  obj?: T | null;
}

const CORE_API: Record<CoreKind, {
  key: () => readonly string[];
  status: () => Promise<CoreApiResult<CoreStatusView>>;
  logs: (n: number) => Promise<CoreApiResult<string[]>>;
  upload: (f: File) => Promise<unknown>;
  download: (u: string, sha256?: string) => Promise<unknown>;
  deleteBinary: () => Promise<unknown>;
}> = {
  naive: {
    key: keys.tunnels.naiveStatus,
    status: tunnelsApi.status,
    logs: tunnelsApi.logs,
    upload: tunnelsApi.upload,
    download: tunnelsApi.download,
    deleteBinary: tunnelsApi.deleteBinary,
  },
  olcrtc: {
    key: keys.tunnels.olcrtcStatus,
    status: tunnelsApi.olcrtcStatus,
    logs: tunnelsApi.olcrtcLogs,
    upload: tunnelsApi.olcrtcUpload,
    download: tunnelsApi.olcrtcDownload,
    deleteBinary: tunnelsApi.olcrtcDeleteBinary,
  },
  qwdtt: {
    key: keys.tunnels.qwdttStatus,
    status: tunnelsApi.qwdttStatus,
    logs: tunnelsApi.qwdttLogs,
    upload: tunnelsApi.qwdttUpload,
    download: tunnelsApi.qwdttDownload,
    deleteBinary: tunnelsApi.qwdttDeleteBinary,
  },
  mieru: {
    key: keys.tunnels.mieruStatus,
    status: tunnelsApi.mieruStatus,
    logs: tunnelsApi.mieruLogs,
    upload: tunnelsApi.mieruUpload,
    download: tunnelsApi.mieruDownload,
    deleteBinary: tunnelsApi.mieruDeleteBinary,
  },
  trusttunnel: {
    key: keys.tunnels.trustTunnelStatus,
    status: tunnelsApi.trustTunnelStatus,
    logs: tunnelsApi.trustTunnelLogs,
    upload: tunnelsApi.trustTunnelUpload,
    download: tunnelsApi.trustTunnelDownload,
    deleteBinary: tunnelsApi.trustTunnelDeleteBinary,
  },
};

function useCoreStatus(kind: CoreKind) {
  const api = CORE_API[kind];
  return useQuery({
    queryKey: api.key(),
    queryFn: api.status,
    refetchInterval: 5000,
  });
}

function BinaryCard({ kind, title }: { kind: CoreKind; title: string }) {
  const { t } = useTranslation();
  const qc = useQueryClient();
  const api = CORE_API[kind];
  const [busy, setBusy] = useState(false);
  const [downloadOpen, setDownloadOpen] = useState(false);
  const [downloadUrl, setDownloadUrl] = useState('');
  const [downloadSha, setDownloadSha] = useState('');
  const { data: res } = useCoreStatus(kind);
  const status = res?.success ? res.obj : undefined;
  const running = !!status?.probe?.running;
  const exists = !!status?.binaryExists;

  const invalidate = useCallback(() => {
    void qc.invalidateQueries({ queryKey: api.key() });
  }, [api, qc]);

  const run = useCallback(async (fn: () => Promise<{ success?: boolean }>) => {
    setBusy(true);
    try {
      await fn();
      invalidate();
    } finally {
      setBusy(false);
    }
  }, [invalidate]);

  const upload = (file: File) => {
    void (async () => {
      setBusy(true);
      try {
        await api.upload(file);
        invalidate();
      } finally {
        setBusy(false);
      }
    })();
    return false;
  };

  const shaTrimmed = downloadSha.trim();
  const shaInvalid = shaTrimmed !== '' && !/^[0-9a-fA-F]{64}$/.test(shaTrimmed);

  const doDownload = async () => {
    if (!downloadUrl.trim() || shaInvalid) return;
    setBusy(true);
    try {
      await api.download(downloadUrl.trim(), shaTrimmed || undefined);
      setDownloadOpen(false);
      setDownloadUrl('');
      setDownloadSha('');
      invalidate();
    } finally {
      setBusy(false);
    }
  };

  const showLogs = async () => {
    const r = await api.logs(200);
    const lines = r.success && Array.isArray(r.obj) ? r.obj : [];
    Modal.info({
      title,
      width: 720,
      content: (
        <pre style={{ maxHeight: 420, overflow: 'auto', fontSize: 12 }}>
          {lines.length ? lines.join('\n') : '(empty)'}
        </pre>
      ),
    });
  };

  const i18nBase = `pages.tunnels.${kind}.binary`;

  return (
    <Card
      size="small"
      style={{ marginBottom: 12 }}
      title={title}
      extra={(
        <Space>
          <Badge
            status={running ? 'success' : 'default'}
            text={running ? t('pages.tunnels.naive.status.running') : t('pages.tunnels.naive.status.stopped')}
          />
          <Tag color={exists ? 'green' : 'red'}>
            {exists ? t(`${i18nBase}.exists`) : t(`${i18nBase}.missing`)}
          </Tag>
        </Space>
      )}
    >
      <Typography.Paragraph type="secondary" style={{ marginBottom: 8 }}>
        <Typography.Text code>{status?.binaryPath || '—'}</Typography.Text>
      </Typography.Paragraph>
      <Space wrap>
        <Upload accept="application/octet-stream,.exe" maxCount={1} showUploadList={false} beforeUpload={upload}>
          <Button icon={<UploadOutlined />} disabled={busy}>{t(`${i18nBase}.upload`)}</Button>
        </Upload>
        <Button icon={<CloudDownloadOutlined />} onClick={() => setDownloadOpen(true)} disabled={busy}>
          {t(`${i18nBase}.download`)}
        </Button>
        <Popconfirm
          title={t(`${i18nBase}.deleteConfirm`)}
          okText={t('delete')}
          cancelText={t('cancel')}
          onConfirm={() => run(() => api.deleteBinary() as Promise<{ success?: boolean }>)}
        >
          <Button icon={<DeleteOutlined />} danger disabled={busy}>{t(`${i18nBase}.delete`)}</Button>
        </Popconfirm>
        <Button icon={<FileSearchOutlined />} onClick={() => void showLogs()}>{t('pages.tunnels.naive.logs')}</Button>
      </Space>
      <Modal
        title={t(`${i18nBase}.download`)}
        open={downloadOpen}
        onCancel={() => setDownloadOpen(false)}
        onOk={() => void doDownload()}
        okText={t(`${i18nBase}.download`)}
        okButtonProps={{ disabled: !downloadUrl.trim() || shaInvalid }}
        confirmLoading={busy}
      >
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input
            placeholder="https://…/binary"
            value={downloadUrl}
            onChange={(e) => setDownloadUrl(e.target.value)}
          />
          <Input
            placeholder={t(`${i18nBase}.sha256Placeholder`)}
            value={downloadSha}
            status={shaInvalid ? 'error' : undefined}
            onChange={(e) => setDownloadSha(e.target.value)}
          />
          <Typography.Text type={shaInvalid ? 'danger' : 'secondary'} style={{ fontSize: 12 }}>
            {shaInvalid ? t(`${i18nBase}.sha256Invalid`) : t(`${i18nBase}.sha256Hint`)}
          </Typography.Text>
        </Space>
      </Modal>
    </Card>
  );
}

function AwgCard() {
  const { t } = useTranslation();
  const [busy, setBusy] = useState(false);
  const { data: statusRes } = useQuery({
    queryKey: ['server', 'status'],
    queryFn: async () => HttpUtil.post<{ awg?: Partial<AwgInfo> }>('/panel/api/server/status', {}, { silent: true }),
    refetchInterval: 5000,
  });
  const awg = statusRes?.obj?.awg;

  const restartIfaces = async () => {
    setBusy(true);
    try {
      await HttpUtil.post('/panel/api/server/restartAwg');
    } finally {
      setBusy(false);
    }
  };

  const rebuildModule = async () => {
    setBusy(true);
    try {
      await HttpUtil.post('/panel/api/server/rebuildAwgModule');
    } finally {
      setBusy(false);
    }
  };

  const loaded = !!awg?.moduleLoaded;
  const ifList = (awg?.ifnames || []).join(', ');

  return (
    <Card
      size="small"
      style={{ marginBottom: 12 }}
      title={t('pages.settings.cores.awgTitle')}
      extra={(
        <Space>
          <Badge
            status={loaded ? (awg?.interfaces ? 'success' : 'default') : 'error'}
            text={loaded
              ? t('pages.settings.cores.awgModuleLoaded')
              : t('pages.settings.cores.awgModuleMissing')}
          />
          {awg?.moduleAwg3 && <Tag color="blue">AWG3</Tag>}
          {awg?.version && <Tag>{awg.version}</Tag>}
        </Space>
      )}
    >
      <Typography.Paragraph type="secondary">
        {t('pages.settings.cores.awgDesc')}
      </Typography.Paragraph>
      {(awg?.errorMsg === 'module_not_loaded' || (awg && !awg.moduleLoaded)) && (
        <Alert type="error" showIcon style={{ marginBottom: 12 }} message={t('pages.index.awgModuleNotLoadedHint')} />
      )}
      {awg?.errorMsg && awg.errorMsg !== 'module_not_loaded' && (
        <Alert type="error" showIcon style={{ marginBottom: 12 }} message={awg.errorMsg} />
      )}
      <Typography.Paragraph>
        {t('pages.index.awgInterfaces', { n: awg?.interfaces ?? 0 })}
        {ifList ? ` · ${ifList}` : ''}
      </Typography.Paragraph>
      <Space wrap>
        <Button icon={<ReloadOutlined />} disabled={busy || !loaded} onClick={() => void restartIfaces()}>
          {t('pages.settings.cores.awgRestart')}
        </Button>
        <Popconfirm
          title={t('pages.settings.cores.awgRebuildConfirm')}
          okText={t('confirm')}
          cancelText={t('cancel')}
          onConfirm={() => void rebuildModule()}
        >
          <Button type="primary" disabled={busy}>
            {t('pages.settings.cores.awgRebuild')}
          </Button>
        </Popconfirm>
      </Space>
    </Card>
  );
}

export default function CoresTab() {
  const { t } = useTranslation();
  return (
    <div>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message={t('pages.settings.cores.banner')}
        description={(
          <>
            {t('pages.settings.cores.bannerDesc')}
            {' '}
            <Link to="/tunnels">{t('pages.settings.cores.openTunnelConfigs')}</Link>
          </>
        )}
      />
      <AwgCard />
      <Divider />
      <Typography.Title level={5}>{t('pages.settings.cores.tunnelBinaries')}</Typography.Title>
      <BinaryCard kind="naive" title="NaiveProxy" />
      <BinaryCard kind="olcrtc" title="olcRTC" />
      <BinaryCard kind="qwdtt" title="qWDTT" />
      <BinaryCard kind="mieru" title="mieru" />
      <BinaryCard kind="trusttunnel" title="TrustTunnel" />
    </div>
  );
}
