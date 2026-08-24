// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Badge,
  Button,
  Card,
  Col,
  Divider,
  Form,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Row,
  Space,
  Tag,
  Typography,
  Upload,
  message,
} from 'antd';
import {
  CaretRightOutlined,
  CloudDownloadOutlined,
  DeleteOutlined,
  FileSearchOutlined,
  PauseOutlined,
  ReloadOutlined,
  SaveOutlined,
  UploadOutlined,
} from '@ant-design/icons';
import { FormProvider } from 'react-hook-form';

import { keys } from '@/api/queryKeys';
import { tunnelsApi } from '@/api/tunnels';
import { FormField, useZodForm } from '@/components/form/rhf';
import { QwdttConfigSchema, type QwdttConfig, type QwdttStatus } from '@/schemas/tunnel';

type ProbeState = 'running' | 'stopped';

function probeState(status: QwdttStatus | undefined): ProbeState {
  if (!status || !status.probe.running) return 'stopped';
  return 'running';
}

export function QwdttCard() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [messageApi, messageContextHolder] = message.useMessage();

  const { data: statusMsg } = useQuery({
    queryKey: keys.tunnels.qwdttStatus(),
    queryFn: () => tunnelsApi.qwdttStatus(),
    refetchInterval: 5000,
  });
  const status = statusMsg?.success ? (statusMsg.obj ?? undefined) : undefined;
  const state = probeState(status);

  const form = useZodForm(QwdttConfigSchema, {
    defaultValues: QwdttConfigSchema.parse({}),
  });
  const loadedRef = useRef(false);
  useEffect(() => {
    if (status && !loadedRef.current) {
      form.reset(status.config);
      loadedRef.current = true;
    }
  }, [status, form]);

  const [logsOpen, setLogsOpen] = useState(false);
  const [logsText, setLogsText] = useState('');
  const [downloadOpen, setDownloadOpen] = useState(false);
  const [downloadUrl, setDownloadUrl] = useState('');
  const [busy, setBusy] = useState(false);

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: keys.tunnels.root() });
  };

  const runAction = async (fn: () => Promise<{ success: boolean; msg: string }>) => {
    setBusy(true);
    try {
      const res = await fn();
      if (!res.success && res.msg) messageApi.error(res.msg);
      invalidate();
    } finally {
      setBusy(false);
    }
  };

  const onSave = form.handleSubmit(async (values: QwdttConfig) => {
    setBusy(true);
    try {
      const res = await tunnelsApi.qwdttSaveConfig(values);
      if (res.success && res.obj) {
        form.reset(res.obj.config);
        messageApi.success(t('pages.tunnels.qwdtt.toasts.saved'));
      } else if (res.msg) {
        messageApi.error(res.msg);
      }
      invalidate();
    } finally {
      setBusy(false);
    }
  });

  const onShowLogs = async () => {
    const res = await tunnelsApi.qwdttLogs(200);
    setLogsText(res.success ? (res.obj ?? []).join('\n') : '');
    setLogsOpen(true);
  };

  const onDownload = async () => {
    if (!downloadUrl.trim()) return;
    setBusy(true);
    try {
      const res = await tunnelsApi.qwdttDownload(downloadUrl.trim());
      if (!res.success && res.msg) messageApi.error(res.msg);
      setDownloadOpen(false);
      setDownloadUrl('');
      invalidate();
    } finally {
      setBusy(false);
    }
  };

  const clientUri = status?.clientUri ?? '';
  const legacyUri = status?.legacyUri ?? '';
  const subJson = status?.subJson ?? '';
  const badge = useMemo(
    () =>
      state === 'running'
        ? { status: 'success' as const, key: 'pages.tunnels.qwdtt.status.running' }
        : { status: 'default' as const, key: 'pages.tunnels.qwdtt.status.stopped' },
    [state],
  );

  return (
    <Card
      style={{ marginTop: 16 }}
      title={t('pages.tunnels.qwdtt.title')}
      extra={
        <Space>
          <Badge status={badge.status} text={t(badge.key)} />
          <Tag color={status?.binaryExists ? 'green' : 'red'}>
            {status?.binaryExists
              ? t('pages.tunnels.qwdtt.binary.exists')
              : t('pages.tunnels.qwdtt.binary.missing')}
          </Tag>
        </Space>
      }
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.tunnels.qwdtt.subtitle')}
      </Typography.Paragraph>

      <Space wrap>
        <Button
          icon={<CaretRightOutlined />}
          disabled={busy}
          onClick={() => runAction(() => tunnelsApi.qwdttStart())}
        >
          {t('pages.tunnels.qwdtt.actions.start')}
        </Button>
        <Button
          icon={<PauseOutlined />}
          disabled={busy}
          onClick={() => runAction(() => tunnelsApi.qwdttStop())}
        >
          {t('pages.tunnels.qwdtt.actions.stop')}
        </Button>
        <Button
          icon={<ReloadOutlined />}
          disabled={busy}
          onClick={() => runAction(() => tunnelsApi.qwdttRestart())}
        >
          {t('pages.tunnels.qwdtt.actions.restart')}
        </Button>
        <Button icon={<FileSearchOutlined />} onClick={() => void onShowLogs()}>
          {t('pages.tunnels.qwdtt.logs')}
        </Button>
      </Space>

      <Divider />

      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Typography.Title level={5}>{t('pages.tunnels.qwdtt.binary.title')}</Typography.Title>
          <Typography.Paragraph type="secondary">
            <Typography.Text code>{status?.binaryPath}</Typography.Text>
          </Typography.Paragraph>
          <Space wrap>
            <Upload
              accept="application/octet-stream,.exe"
              maxCount={1}
              showUploadList={false}
              beforeUpload={(file) => {
                void (async () => {
                  setBusy(true);
                  try {
                    const res = await tunnelsApi.qwdttUpload(file);
                    if (!res.success && res.msg) messageApi.error(res.msg);
                    invalidate();
                  } finally {
                    setBusy(false);
                  }
                })();
                return false;
              }}
            >
              <Button icon={<UploadOutlined />} disabled={busy}>
                {t('pages.tunnels.qwdtt.binary.upload')}
              </Button>
            </Upload>
            <Button icon={<CloudDownloadOutlined />} onClick={() => setDownloadOpen(true)}>
              {t('pages.tunnels.qwdtt.binary.download')}
            </Button>
            <Popconfirm
              title={t('pages.tunnels.qwdtt.binary.deleteConfirm')}
              okText={t('delete')}
              cancelText={t('cancel')}
              onConfirm={() => runAction(() => tunnelsApi.qwdttDeleteBinary())}
            >
              <Button icon={<DeleteOutlined />} danger disabled={busy}>
                {t('pages.tunnels.qwdtt.binary.delete')}
              </Button>
            </Popconfirm>
          </Space>

          {clientUri !== '' && (
            <>
              <Divider />
              <Typography.Title level={5}>{t('pages.tunnels.qwdtt.clientUri')}</Typography.Title>
              <Typography.Paragraph copyable={{ text: clientUri }}>
                <Typography.Text code style={{ wordBreak: 'break-all' }}>
                  {clientUri}
                </Typography.Text>
              </Typography.Paragraph>
            </>
          )}
          {legacyUri !== '' && (
            <>
              <Typography.Title level={5}>{t('pages.tunnels.qwdtt.legacyUri')}</Typography.Title>
              <Typography.Paragraph copyable={{ text: legacyUri }}>
                <Typography.Text code style={{ wordBreak: 'break-all' }}>
                  {legacyUri}
                </Typography.Text>
              </Typography.Paragraph>
            </>
          )}
          {subJson !== '' && (
            <>
              <Typography.Title level={5}>{t('pages.tunnels.qwdtt.subJson')}</Typography.Title>
              <Typography.Paragraph copyable={{ text: subJson }}>
                <pre style={{ maxHeight: 180, overflow: 'auto', fontSize: 11 }}>{subJson}</pre>
              </Typography.Paragraph>
            </>
          )}
        </Col>

        <Col xs={24} md={16}>
          <FormProvider {...form}>
            <Form layout="vertical" onFinish={() => void onSave()}>
              <Row gutter={16}>
                <Col xs={24} sm={12}>
                  <FormField
                    name="remark"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.remark')}
                  >
                    <Input />
                  </FormField>
                </Col>
                <Col xs={24} sm={12}>
                  <FormField
                    name="subHost"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.subHost')}
                    tooltip={t('pages.tunnels.qwdtt.form.subHostTip')}
                  >
                    <Input placeholder="203.0.113.10:56000" />
                  </FormField>
                </Col>
              </Row>

              <Row gutter={16}>
                <Col xs={24} sm={12}>
                  <FormField
                    name="listenAddr"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.listenAddr')}
                    tooltip={t('pages.tunnels.qwdtt.form.listenAddrTip')}
                    required
                  >
                    <Input placeholder="0.0.0.0:56000" />
                  </FormField>
                </Col>
                <Col xs={12} sm={6}>
                  <FormField
                    name="wgPort"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.wgPort')}
                    required
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </FormField>
                </Col>
                <Col xs={12} sm={6}>
                  <FormField
                    name="dns"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.dns')}
                    required
                  >
                    <Input placeholder="8.8.8.8" />
                  </FormField>
                </Col>
              </Row>

              <FormField
                name="password"
                control={form.control}
                label={t('pages.tunnels.qwdtt.form.password')}
                tooltip={t('pages.tunnels.qwdtt.form.passwordTip')}
              >
                <Input.Password autoComplete="new-password" placeholder="auto on save" />
              </FormField>

              <FormField
                name="vkHashes"
                control={form.control}
                label={t('pages.tunnels.qwdtt.form.vkHashes')}
                tooltip={t('pages.tunnels.qwdtt.form.vkHashesTip')}
              >
                <Input.TextArea rows={2} placeholder="hash1,hash2" />
              </FormField>

              <Row gutter={16}>
                <Col xs={24} sm={12}>
                  <FormField
                    name="listenRaw"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.listenRaw')}
                    tooltip={t('pages.tunnels.qwdtt.form.listenRawTip')}
                  >
                    <Input placeholder="0.0.0.0:56003" />
                  </FormField>
                </Col>
                <Col xs={24} sm={12}>
                  <FormField
                    name="configDir"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.configDir')}
                    tooltip={t('pages.tunnels.qwdtt.form.configDirTip')}
                  >
                    <Input placeholder="(default: bin/tunnel/qwdtt-data)" />
                  </FormField>
                </Col>
              </Row>

              <Row gutter={16}>
                <Col xs={12} sm={8}>
                  <FormField
                    name="clientPort"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.clientPort')}
                  >
                    <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                  </FormField>
                </Col>
                <Col xs={12} sm={8}>
                  <FormField
                    name="workers"
                    control={form.control}
                    label={t('pages.tunnels.qwdtt.form.workers')}
                  >
                    <InputNumber min={1} max={64} style={{ width: '100%' }} />
                  </FormField>
                </Col>
              </Row>

              <Space>
                <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={busy}>
                  {t('pages.tunnels.qwdtt.form.save')}
                </Button>
              </Space>
            </Form>
          </FormProvider>
        </Col>
      </Row>

      <Modal
        title={t('pages.tunnels.qwdtt.logsTitle')}
        open={logsOpen}
        footer={null}
        width={720}
        onCancel={() => setLogsOpen(false)}
      >
        <Typography.Paragraph>
          <pre style={{ maxHeight: 420, overflow: 'auto', fontSize: 12 }}>
            {logsText || t('pages.tunnels.qwdtt.logsEmpty')}
          </pre>
        </Typography.Paragraph>
      </Modal>

      <Modal
        title={t('pages.tunnels.qwdtt.binary.download')}
        open={downloadOpen}
        onOk={() => void onDownload()}
        onCancel={() => setDownloadOpen(false)}
        confirmLoading={busy}
      >
        <Input
          placeholder="https://…"
          value={downloadUrl}
          onChange={(e) => setDownloadUrl(e.target.value)}
        />
      </Modal>
    </Card>
  );
}
