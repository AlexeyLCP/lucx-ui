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
  Select,
  Space,
  Switch,
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
import { FormProvider, useWatch } from 'react-hook-form';

import { keys } from '@/api/queryKeys';
import { tunnelsApi } from '@/api/tunnels';
import { FormField, useZodForm } from '@/components/form/rhf';
import { OlcrtcConfigSchema, type OlcrtcConfig, type OlcrtcStatus } from '@/schemas/tunnel';

type ProbeState = 'running' | 'stopped';

function probeState(status: OlcrtcStatus | undefined): ProbeState {
  if (!status || !status.probe.running) return 'stopped';
  return 'running';
}

async function generateCryptoKey(): Promise<string> {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

export function OlcrtcCard() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [messageApi, messageContextHolder] = message.useMessage();

  const { data: statusMsg } = useQuery({
    queryKey: keys.tunnels.olcrtcStatus(),
    queryFn: () => tunnelsApi.olcrtcStatus(),
    refetchInterval: 5000,
  });
  const status = statusMsg?.success ? (statusMsg.obj ?? undefined) : undefined;
  const state = probeState(status);

  const form = useZodForm(OlcrtcConfigSchema, {
    defaultValues: OlcrtcConfigSchema.parse({}),
  });
  const loadedRef = useRef(false);
  useEffect(() => {
    if (status && !loadedRef.current) {
      form.reset(status.config);
      loadedRef.current = true;
    }
  }, [status, form]);

  const provider = useWatch({ control: form.control, name: 'provider' });
  const transport = useWatch({ control: form.control, name: 'transport' });

  useEffect(() => {
    if (provider === 'telemost' && transport !== 'vp8channel') {
      form.setValue('transport', 'vp8channel', { shouldDirty: true });
    }
  }, [provider, transport, form]);

  const [logsOpen, setLogsOpen] = useState(false);
  const [logsText, setLogsText] = useState('');
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewText, setPreviewText] = useState('');
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

  const onSave = form.handleSubmit(async (values: OlcrtcConfig) => {
    setBusy(true);
    try {
      const res = await tunnelsApi.olcrtcSaveConfig(values);
      if (res.success && res.obj) {
        form.reset(res.obj.config);
        messageApi.success(t('pages.tunnels.olcrtc.toasts.saved'));
      } else if (res.msg) {
        messageApi.error(res.msg);
      }
      invalidate();
    } finally {
      setBusy(false);
    }
  });

  const onPreview = async () => {
    const values = form.getValues();
    const res = await tunnelsApi.olcrtcPreview(values);
    if (res.success && res.obj) {
      setPreviewText(res.obj.yaml);
      setPreviewOpen(true);
    } else if (res.msg) {
      messageApi.error(res.msg);
    }
  };

  const onShowLogs = async () => {
    const res = await tunnelsApi.olcrtcLogs(200);
    setLogsText(res.success ? (res.obj ?? []).join('\n') : '');
    setLogsOpen(true);
  };

  const onDownload = async () => {
    if (!downloadUrl.trim()) return;
    setBusy(true);
    try {
      const res = await tunnelsApi.olcrtcDownload(downloadUrl.trim());
      if (!res.success && res.msg) messageApi.error(res.msg);
      setDownloadOpen(false);
      setDownloadUrl('');
      invalidate();
    } finally {
      setBusy(false);
    }
  };

  const clientUri = status?.clientUri ?? '';
  const badge = useMemo(
    () =>
      state === 'running'
        ? { status: 'success' as const, key: 'pages.tunnels.olcrtc.status.running' }
        : { status: 'default' as const, key: 'pages.tunnels.olcrtc.status.stopped' },
    [state],
  );

  const transportOptions =
    provider === 'telemost'
      ? [{ value: 'vp8channel', label: 'vp8channel' }]
      : [
          { value: 'datachannel', label: 'datachannel' },
          { value: 'vp8channel', label: 'vp8channel' },
        ];

  return (
    <Card
      style={{ marginTop: 16 }}
      title={t('pages.tunnels.olcrtc.title')}
      extra={
        <Space>
          <Badge status={badge.status} text={t(badge.key)} />
          <Tag color={status?.binaryExists ? 'green' : 'red'}>
            {status?.binaryExists
              ? t('pages.tunnels.olcrtc.binary.exists')
              : t('pages.tunnels.olcrtc.binary.missing')}
          </Tag>
        </Space>
      }
    >
      {messageContextHolder}
      <Typography.Paragraph type="secondary">
        {t('pages.tunnels.olcrtc.subtitle')}
      </Typography.Paragraph>

      <Space wrap>
        <Button
          icon={<CaretRightOutlined />}
          disabled={busy}
          onClick={() => runAction(() => tunnelsApi.olcrtcStart())}
        >
          {t('pages.tunnels.olcrtc.actions.start')}
        </Button>
        <Button
          icon={<PauseOutlined />}
          disabled={busy}
          onClick={() => runAction(() => tunnelsApi.olcrtcStop())}
        >
          {t('pages.tunnels.olcrtc.actions.stop')}
        </Button>
        <Button
          icon={<ReloadOutlined />}
          disabled={busy}
          onClick={() => runAction(() => tunnelsApi.olcrtcRestart())}
        >
          {t('pages.tunnels.olcrtc.actions.restart')}
        </Button>
        <Button icon={<FileSearchOutlined />} onClick={() => void onShowLogs()}>
          {t('pages.tunnels.olcrtc.logs')}
        </Button>
      </Space>

      <Divider />

      <Row gutter={16}>
        <Col xs={24} md={8}>
          <Typography.Title level={5}>{t('pages.tunnels.olcrtc.binary.title')}</Typography.Title>
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
                    const res = await tunnelsApi.olcrtcUpload(file);
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
                {t('pages.tunnels.olcrtc.binary.upload')}
              </Button>
            </Upload>
            <Button icon={<CloudDownloadOutlined />} onClick={() => setDownloadOpen(true)}>
              {t('pages.tunnels.olcrtc.binary.download')}
            </Button>
            <Popconfirm
              title={t('pages.tunnels.olcrtc.binary.deleteConfirm')}
              okText={t('delete')}
              cancelText={t('cancel')}
              onConfirm={() => runAction(() => tunnelsApi.olcrtcDeleteBinary())}
            >
              <Button icon={<DeleteOutlined />} danger disabled={busy}>
                {t('pages.tunnels.olcrtc.binary.delete')}
              </Button>
            </Popconfirm>
          </Space>

          {clientUri !== '' && (
            <>
              <Divider />
              <Typography.Title level={5}>{t('pages.tunnels.olcrtc.clientUri')}</Typography.Title>
              <Typography.Paragraph copyable={{ text: clientUri }}>
                <Typography.Text code style={{ wordBreak: 'break-all' }}>
                  {clientUri}
                </Typography.Text>
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
                    label={t('pages.tunnels.olcrtc.form.remark')}
                  >
                    <Input />
                  </FormField>
                </Col>
                <Col xs={24} sm={12}>
                  <FormField
                    name="provider"
                    control={form.control}
                    label={t('pages.tunnels.olcrtc.form.provider')}
                    tooltip={t('pages.tunnels.olcrtc.form.providerTip')}
                    required
                  >
                    <Select
                      options={[
                        { value: 'jitsi', label: 'Jitsi' },
                        { value: 'telemost', label: 'Yandex Telemost' },
                        { value: 'wbstream', label: 'WB Stream' },
                      ]}
                    />
                  </FormField>
                </Col>
              </Row>

              <FormField
                name="roomId"
                control={form.control}
                label={t('pages.tunnels.olcrtc.form.roomId')}
                tooltip={t('pages.tunnels.olcrtc.form.roomIdTip')}
                required
              >
                <Input placeholder="https://meet.jit.si/my-room" />
              </FormField>

              <FormField
                name="cryptoKey"
                control={form.control}
                label={t('pages.tunnels.olcrtc.form.cryptoKey')}
                tooltip={t('pages.tunnels.olcrtc.form.cryptoKeyTip')}
                extra={
                  <Button
                    size="small"
                    type="link"
                    style={{ paddingLeft: 0 }}
                    onClick={() => {
                      void generateCryptoKey().then((k) =>
                        form.setValue('cryptoKey', k, { shouldDirty: true }),
                      );
                    }}
                  >
                    {t('pages.tunnels.olcrtc.form.genKey')}
                  </Button>
                }
              >
                <Input.Password
                  autoComplete="new-password"
                  placeholder="64 hex chars (auto on save)"
                />
              </FormField>

              <Row gutter={16}>
                <Col xs={24} sm={12}>
                  <FormField
                    name="transport"
                    control={form.control}
                    label={t('pages.tunnels.olcrtc.form.transport')}
                    tooltip={t('pages.tunnels.olcrtc.form.transportTip')}
                    required
                  >
                    <Select options={transportOptions} />
                  </FormField>
                </Col>
                <Col xs={24} sm={12}>
                  <FormField
                    name="dns"
                    control={form.control}
                    label={t('pages.tunnels.olcrtc.form.dns')}
                    tooltip={t('pages.tunnels.olcrtc.form.dnsTip')}
                    required
                  >
                    <Input placeholder="8.8.8.8:53" />
                  </FormField>
                </Col>
              </Row>

              {transport === 'vp8channel' && (
                <Row gutter={16}>
                  <Col xs={12} sm={8}>
                    <FormField
                      name="vp8Fps"
                      control={form.control}
                      label={t('pages.tunnels.olcrtc.form.vp8Fps')}
                    >
                      <InputNumber min={1} max={120} style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col xs={12} sm={8}>
                    <FormField
                      name="vp8Batch"
                      control={form.control}
                      label={t('pages.tunnels.olcrtc.form.vp8Batch')}
                    >
                      <InputNumber min={1} max={64} style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                </Row>
              )}

              <FormField
                name="debug"
                control={form.control}
                label={t('pages.tunnels.olcrtc.form.debug')}
                valueProp="checked"
              >
                <Switch />
              </FormField>

              <Space>
                <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={busy}>
                  {t('pages.tunnels.olcrtc.form.save')}
                </Button>
                <Button onClick={() => void onPreview()}>
                  {t('pages.tunnels.olcrtc.form.preview')}
                </Button>
              </Space>
            </Form>
          </FormProvider>
        </Col>
      </Row>

      <Modal
        title={t('pages.tunnels.olcrtc.logsTitle')}
        open={logsOpen}
        footer={null}
        width={720}
        onCancel={() => setLogsOpen(false)}
      >
        <Typography.Paragraph>
          <pre style={{ maxHeight: 420, overflow: 'auto', fontSize: 12 }}>
            {logsText || t('pages.tunnels.olcrtc.logsEmpty')}
          </pre>
        </Typography.Paragraph>
      </Modal>

      <Modal
        title={t('pages.tunnels.olcrtc.form.preview')}
        open={previewOpen}
        footer={null}
        width={720}
        onCancel={() => setPreviewOpen(false)}
      >
        <Typography.Paragraph>
          <pre style={{ maxHeight: 420, overflow: 'auto', fontSize: 12 }}>{previewText}</pre>
        </Typography.Paragraph>
      </Modal>

      <Modal
        title={t('pages.tunnels.olcrtc.binary.download')}
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
