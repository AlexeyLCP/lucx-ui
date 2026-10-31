import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery } from '@tanstack/react-query';
import {
  Alert,
  Button,
  Card,
  ConfigProvider,
  Layout,
  Popconfirm,
  Space,
  Spin,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import { DeleteOutlined, ReloadOutlined, UploadOutlined } from '@ant-design/icons';

import { pluginsApi } from '@/api/plugins';
import AppSidebar from '@/layouts/AppSidebar';
import { useTheme } from '@/hooks/useTheme';

export default function PluginManagerPage() {
  const { t } = useTranslation();
  const { antdThemeConfig } = useTheme();
  const [busy, setBusy] = useState(false);
  const [messageApi, messageContextHolder] = message.useMessage();
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
              message="Plugins are not sandboxed. They run with the panel's privileges and may access its environment, database and secrets. Install only trusted plugins."
              style={{ marginBottom: 16 }}
            />
            <Space wrap style={{ marginBottom: 16 }}>
              <Upload accept=".zip" showUploadList={false} beforeUpload={upload}>
                <Button icon={<UploadOutlined aria-hidden />} disabled={busy}>
                  {t('pages.plugins.upload')}
                </Button>
              </Upload>
              <Button
                icon={<ReloadOutlined aria-hidden />}
                onClick={() => void refetch()}
                loading={isLoading}
              >
                {t('pages.plugins.refresh')}
              </Button>
            </Space>
            <Spin spinning={isLoading}>
              <Space direction="vertical" style={{ width: '100%' }} size={12}>
                {plugins.length === 0 && (
                  <Card>
                    <Typography.Text type="secondary">{t('pages.plugins.empty')}</Typography.Text>
                  </Card>
                )}
                {plugins.map((plugin) => (
                  <Card
                    key={plugin.manifest.id}
                    size="small"
                    title={
                      <Space>
                        {plugin.manifest.name}
                        <Tag>{plugin.manifest.version}</Tag>
                        <Tag color={plugin.running ? 'green' : 'default'}>
                          {plugin.running ? t('pages.plugins.running') : t('pages.plugins.stopped')}
                        </Tag>
                      </Space>
                    }
                  >
                    <Typography.Paragraph type="secondary">
                      {plugin.manifest.id} · {plugin.manifest.type}
                    </Typography.Paragraph>
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
                ))}
              </Space>
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
