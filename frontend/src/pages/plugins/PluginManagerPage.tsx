import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import {
  Alert,
  Button,
  Card,
  ConfigProvider,
  Descriptions,
  Input,
  Layout,
  Modal,
  Popconfirm,
  Space,
  Spin,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import { DeleteOutlined, LinkOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';

import { pluginsApi, type PluginManifest } from '@/api/plugins';
import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';

export default function PluginManagerPage() {
  const { t } = useTranslation();
  const { antdThemeConfig } = useTheme();
  const [busy, setBusy] = useState(false);
  const [messageApi, messageContextHolder] = message.useMessage();
  const [linkOpen, setLinkOpen] = useState(false);
  const [linkUrl, setLinkUrl] = useState('');
  const [preview, setPreview] = useState<PluginManifest | null>(null);
  const [checking, setChecking] = useState(false);
  const { data, isLoading, refetch } = useQuery({
    queryKey: ['plugins'],
    queryFn: pluginsApi.list,
    refetchInterval: 5000,
  });
  const plugins = data?.success ? (data.obj ?? []) : [];
  const run = async (fn: () => Promise<unknown>) => {
    setBusy(true);
    try {
      await fn();
      await refetch();
    } catch {
      void messageApi.error(t('pages.plugins.operationFailed'));
    } finally {
      setBusy(false);
    }
  };
  const upload = (file: File) => {
    void run(() => pluginsApi.install(file));
    return false;
  };
  const openLinkDialog = () => {
    setLinkUrl('');
    setPreview(null);
    setLinkOpen(true);
  };
  const checkLink = async () => {
    setChecking(true);
    try {
      const res = await pluginsApi.previewFromLink(linkUrl.trim());
      setPreview(res.success ? (res.obj ?? null) : null);
      if (!res.success) {
        void messageApi.error(res.msg || t('pages.plugins.linkPreviewFailed'));
      }
    } catch {
      void messageApi.error(t('pages.plugins.linkPreviewFailed'));
    } finally {
      setChecking(false);
    }
  };
  const installLink = async () => {
    if (!preview) return;
    setBusy(true);
    try {
      await pluginsApi.installFromLink(linkUrl.trim());
      await refetch();
      setLinkOpen(false);
    } catch {
      void messageApi.error(t('pages.plugins.operationFailed'));
    } finally {
      setBusy(false);
    }
  };

  return (
    <ConfigProvider theme={antdThemeConfig}>
      {messageContextHolder}
      <Layout className="page-shell">
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Typography.Title level={3}>{t('pages.plugins.title')}</Typography.Title>
            <Alert
              type="warning"
              showIcon
              message={t('pages.plugins.sandboxWarning')}
              description={
                <Space split>
                  <Typography.Link
                    href="https://github.com/AlexeyLCP/lucx-ui/blob/main/docs/plugin-manager.md"
                    target="_blank"
                  >
                    {t('pages.plugins.docsLink')}
                  </Typography.Link>
                  <Typography.Link
                    href="https://github.com/AlexeyLCP/lucx-ui/blob/main/docs/plugin-prompt.md"
                    target="_blank"
                  >
                    {t('pages.plugins.promptLink')}
                  </Typography.Link>
                </Space>
              }
              style={{ marginBottom: 16 }}
            />
            <Space wrap style={{ marginBottom: 16 }}>
              <Upload accept=".zip" showUploadList={false} beforeUpload={upload}>
                <Button icon={<UploadOutlined aria-hidden />} disabled={busy}>
                  {t('pages.plugins.upload')}
                </Button>
              </Upload>
              <Button icon={<LinkOutlined aria-hidden />} href="#" onClick={openLinkDialog}>
                {t('pages.plugins.installByLink')}
              </Button>
              <Button
                icon={<ReloadOutlined aria-hidden />}
                onClick={() => void refetch()}
                loading={isLoading}
              >
                {t('pages.plugins.refresh')}
              </Button>
            </Space>
            <Modal
              open={linkOpen}
              title={t('pages.plugins.installByLink')}
              onCancel={() => setLinkOpen(false)}
              footer={
                <Space>
                  <Button onClick={() => setLinkOpen(false)}>{t('pages.plugins.cancel')}</Button>
                  <Button
                    type="primary"
                    danger={!!preview}
                    disabled={!preview || busy}
                    onClick={() => void installLink()}
                  >
                    {t('pages.plugins.linkInstall')}
                  </Button>
                </Space>
              }
            >
              <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
                <Input
                  aria-label={t('pages.plugins.installByLink')}
                  placeholder={t('pages.plugins.linkPlaceholder')}
                  value={linkUrl}
                  onChange={(e) => setLinkUrl(e.target.value)}
                  allowClear
                />
                <Button
                  loading={checking}
                  disabled={!linkUrl.trim()}
                  onClick={() => void checkLink()}
                >
                  {t('pages.plugins.linkPreview')}
                </Button>
              </Space.Compact>
              {preview && (
                <>
                  <Alert
                    type="warning"
                    showIcon
                    message={t('pages.plugins.linkWarning')}
                    style={{ marginBottom: 12 }}
                  />
                  <Descriptions
                    size="small"
                    column={1}
                    items={[
                      { key: 'id', label: 'id', children: preview.id },
                      { key: 'name', label: 'name', children: preview.name },
                      { key: 'version', label: 'version', children: preview.version },
                      { key: 'type', label: 'type', children: preview.type },
                      { key: 'entry', label: 'entrypoint', children: preview.entrypoint },
                      {
                        key: 'perm',
                        label: 'permissions',
                        children: preview.permissions ? JSON.stringify(preview.permissions) : '—',
                      },
                      {
                        key: 'res',
                        label: 'resources',
                        children: preview.resources ? JSON.stringify(preview.resources) : '—',
                      },
                      {
                        key: 'health',
                        label: 'health',
                        children: preview.health ? JSON.stringify(preview.health) : '—',
                      },
                    ]}
                  />
                </>
              )}
            </Modal>
            <Spin spinning={isLoading}>
              <Space direction="vertical" style={{ width: '100%' }} size={12}>
                {plugins.length === 0 && (
                  <Card>
                    <Typography.Text type="secondary">{t('pages.plugins.empty')}</Typography.Text>
                  </Card>
                )}
                {plugins.map((plugin) => {
                  const { enforcement } = plugin;
                  const asked =
                    enforcement.networkIsolated ||
                    (enforcement.memoryLimitMB ?? 0) > 0 ||
                    (enforcement.cpuLimitPercent ?? 0) > 0;
                  return (
                    <Card
                      key={plugin.manifest.id}
                      size="small"
                      title={
                        <Space>
                          {plugin.manifest.name}
                          <Tag>{plugin.manifest.version}</Tag>
                          <Tag color={plugin.running ? 'green' : 'default'}>
                            {plugin.running
                              ? t('pages.plugins.running')
                              : t('pages.plugins.stopped')}
                          </Tag>
                          {plugin.manifest.health && (
                            <Tag
                              color={
                                plugin.health?.status === 'pass'
                                  ? 'green'
                                  : plugin.health?.status === 'fail'
                                    ? 'red'
                                    : 'default'
                              }
                            >
                              {t(
                                `pages.plugins.health${plugin.health?.status === 'pass' ? 'Pass' : plugin.health?.status === 'fail' ? 'Fail' : 'Unknown'}`,
                              )}
                            </Tag>
                          )}
                          {(plugin.manifest.events?.length ?? 0) > 0 && (
                            <Tag color="cyan">
                              {t('pages.plugins.eventsCount', {
                                count: plugin.manifest.events?.length,
                              })}
                            </Tag>
                          )}
                          {(plugin.manifest.permissions?.sub?.length ?? 0) > 0 && (
                            <Tag color="cyan">{t('pages.plugins.subAfter')}</Tag>
                          )}
                        </Space>
                      }
                    >
                      <Typography.Paragraph type="secondary">
                        {plugin.manifest.id} · {plugin.manifest.type}
                      </Typography.Paragraph>
                      {asked && (
                        <Space wrap style={{ marginBottom: 8 }}>
                          {enforcement.networkIsolated && (
                            <Tag color="geekblue">{t('pages.plugins.networkIsolated')}</Tag>
                          )}
                          {(enforcement.memoryLimitMB ?? 0) > 0 && (
                            <Tag color="geekblue">
                              {t('pages.plugins.memoryLimit', { count: enforcement.memoryLimitMB })}
                            </Tag>
                          )}
                          {(enforcement.cpuLimitPercent ?? 0) > 0 && (
                            <Tag color={enforcement.cpuEnforced ? 'geekblue' : 'orange'}>
                              {t('pages.plugins.cpuLimit', { count: enforcement.cpuLimitPercent })}
                            </Tag>
                          )}
                          {plugin.runner && (
                            <Tag color={plugin.runner === 'unit' ? 'geekblue' : 'default'}>
                              {plugin.runner === 'unit'
                                ? t('pages.plugins.runnerUnit')
                                : t('pages.plugins.runnerProcess')}
                            </Tag>
                          )}
                          {asked && !enforcement.enforceable && (
                            <Tag color="orange">{t('pages.plugins.notEnforcedHere')}</Tag>
                          )}
                        </Space>
                      )}
                      <Typography.Paragraph
                        copyable={{ text: plugin.binaryPath }}
                        ellipsis={{ rows: 1 }}
                      >
                        {plugin.binaryPath}
                      </Typography.Paragraph>
                      <Space wrap>
                        <Button
                          disabled={busy}
                          onClick={() =>
                            void run(() =>
                              plugin.running
                                ? pluginsApi.stop(plugin.manifest.id)
                                : pluginsApi.start(plugin.manifest.id),
                            )
                          }
                        >
                          {plugin.running ? t('pages.plugins.stop') : t('pages.plugins.start')}
                        </Button>
                        <Button
                          disabled={busy}
                          onClick={() => void run(() => pluginsApi.restart(plugin.manifest.id))}
                        >
                          {t('pages.plugins.restart')}
                        </Button>
                        <Popconfirm
                          title={t('pages.plugins.deleteConfirm')}
                          onConfirm={() => void run(() => pluginsApi.remove(plugin.manifest.id))}
                        >
                          <Button danger icon={<DeleteOutlined aria-hidden />} disabled={busy}>
                            {t('pages.plugins.delete')}
                          </Button>
                        </Popconfirm>
                      </Space>
                    </Card>
                  );
                })}
              </Space>
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
