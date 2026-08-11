// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useMemo, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
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
import { useOutboundTags } from '@/api/queries/useOutboundTags';
import { tunnelsApi } from '@/api/tunnels';
import { FormField, useZodForm } from '@/components/form/rhf';
import { QrPanel } from '@/pages/inbounds/qr';
import { NaiveConfigSchema, type NaiveConfig, type NaiveStatus } from '@/schemas/tunnel';
import { OlcrtcCard } from '@/pages/tunnels/OlcrtcCard';
import { QwdttCard } from '@/pages/tunnels/QwdttCard';

// generateNaivePassword returns 18 random bytes as 24 chars of URL-safe
// base64 — strong enough for a basic_auth secret, safe in share links.
function generateNaivePassword(): string {
  const bytes = new Uint8Array(18);
  crypto.getRandomValues(bytes);
  let bin = '';
  bytes.forEach((b) => {
    bin += String.fromCharCode(b);
  });
  return btoa(bin).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

type ProbeState = 'running' | 'unresponsive' | 'starting' | 'stopped';

function probeState(status: NaiveStatus | undefined): ProbeState {
  if (!status || !status.probe.running) return 'stopped';
  if (!status.probe.listening) return 'starting';
  if (!status.probe.responding) return 'unresponsive';
  return 'running';
}

const PROBE_BADGE: Record<ProbeState, { status: 'success' | 'warning' | 'processing' | 'default'; key: string }> = {
  running: { status: 'success', key: 'pages.tunnels.naive.status.running' },
  unresponsive: { status: 'warning', key: 'pages.tunnels.naive.status.unresponsive' },
  starting: { status: 'processing', key: 'pages.tunnels.naive.status.starting' },
  stopped: { status: 'default', key: 'pages.tunnels.naive.status.stopped' },
};

export default function TunnelsPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [messageApi, messageContextHolder] = message.useMessage();

  const { data: statusMsg } = useQuery({
    queryKey: keys.tunnels.naiveStatus(),
    queryFn: () => tunnelsApi.status(),
    refetchInterval: 5000,
  });
  const status = statusMsg?.success ? (statusMsg.obj ?? undefined) : undefined;
  const state = probeState(status);

  const form = useZodForm(NaiveConfigSchema, {
    defaultValues: NaiveConfigSchema.parse({}),
  });
  const loadedRef = useRef(false);
  useEffect(() => {
    if (status && !loadedRef.current) {
      form.reset(status.config);
      loadedRef.current = true;
    }
  }, [status, form]);

  const useRaw = useWatch({ control: form.control, name: 'useRawConfig' });
  const useAcme = useWatch({ control: form.control, name: 'useAcme' });
  const routeThroughXray = useWatch({ control: form.control, name: 'routeThroughXray' });
  const { data: outboundTags } = useOutboundTags();

  const [logsOpen, setLogsOpen] = useState(false);
  const [logsText, setLogsText] = useState('');
  const [previewOpen, setPreviewOpen] = useState(false);
  const [previewText, setPreviewText] = useState('');
  const [downloadOpen, setDownloadOpen] = useState(false);
  const [downloadUrl, setDownloadUrl] = useState('');
  const [qrOpen, setQrOpen] = useState(false);
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

  const onSave = form.handleSubmit(async (values: NaiveConfig) => {
    setBusy(true);
    try {
      const res = await tunnelsApi.saveConfig(values);
      if (res.success && res.obj) {
        form.reset(res.obj.config);
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
    const res = await tunnelsApi.preview(values);
    if (res.success && res.obj) {
      setPreviewText(res.obj.caddyfile);
      setPreviewOpen(true);
    } else if (res.msg) {
      messageApi.error(res.msg);
    }
  };

  const onValidateRaw = async () => {
    const text = form.getValues('rawConfig');
    const res = await tunnelsApi.validate(text);
    if (res.success) {
      messageApi.success(t('pages.tunnels.naive.validOk'));
    } else if (res.msg) {
      messageApi.error(res.msg);
    }
  };

  const onShowLogs = async () => {
    const res = await tunnelsApi.logs(200);
    setLogsText(res.success ? (res.obj ?? []).join('\n') : '');
    setLogsOpen(true);
  };

  const onDownload = async () => {
    if (!downloadUrl.trim()) return;
    setBusy(true);
    try {
      const res = await tunnelsApi.download(downloadUrl.trim());
      if (!res.success && res.msg) messageApi.error(res.msg);
      setDownloadOpen(false);
      setDownloadUrl('');
      invalidate();
    } finally {
      setBusy(false);
    }
  };

  const clientUrl = status?.clientUrl ?? '';
  const badge = useMemo(() => PROBE_BADGE[state], [state]);

  return (
    <div>
      {messageContextHolder}
      <Typography.Title level={3}>{t('pages.tunnels.title')}</Typography.Title>
      <Typography.Paragraph type="secondary">{t('pages.tunnels.subtitle')}</Typography.Paragraph>

      <Alert
        type="info"
        showIcon
        message={t('pages.tunnels.inboundMovedTitle')}
        description={t('pages.tunnels.inboundMovedDesc')}
        style={{ marginBottom: 16 }}
      />

      <Card
        title={t('pages.tunnels.naive.title')}
        extra={
          <Space>
            <Badge status={badge.status} text={t(badge.key)} />
            <Tag color={status?.binaryExists ? 'green' : 'red'}>
              {status?.binaryExists ? t('pages.tunnels.naive.binary.exists') : t('pages.tunnels.naive.binary.missing')}
            </Tag>
          </Space>
        }
      >

        <Space wrap>
          <Button
            icon={<CaretRightOutlined />}
            disabled={busy}
            onClick={() => runAction(() => tunnelsApi.start())}
          >
            {t('pages.tunnels.naive.actions.start')}
          </Button>
          <Button
            icon={<PauseOutlined />}
            disabled={busy}
            onClick={() => runAction(() => tunnelsApi.stop())}
          >
            {t('pages.tunnels.naive.actions.stop')}
          </Button>
          <Button
            icon={<ReloadOutlined />}
            disabled={busy}
            onClick={() => runAction(() => tunnelsApi.restart())}
          >
            {t('pages.tunnels.naive.actions.restart')}
          </Button>
          <Button icon={<FileSearchOutlined />} onClick={() => void onShowLogs()}>
            {t('pages.tunnels.naive.logs')}
          </Button>
        </Space>

        <Divider />

        <Row gutter={16}>
          <Col xs={24} md={8}>
            <Typography.Title level={5}>{t('pages.tunnels.naive.binary.title')}</Typography.Title>
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
                      const res = await tunnelsApi.upload(file);
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
                  {t('pages.tunnels.naive.binary.upload')}
                </Button>
              </Upload>
              <Button icon={<CloudDownloadOutlined />} onClick={() => setDownloadOpen(true)}>
                {t('pages.tunnels.naive.binary.download')}
              </Button>
              <Popconfirm
                title={t('pages.tunnels.naive.binary.deleteConfirm')}
                okText={t('delete')}
                cancelText={t('cancel')}
                onConfirm={() => runAction(() => tunnelsApi.deleteBinary())}
              >
                <Button icon={<DeleteOutlined />} danger disabled={busy}>
                  {t('pages.tunnels.naive.binary.delete')}
                </Button>
              </Popconfirm>
            </Space>

            {clientUrl !== '' && (
              <>
                <Divider />
                <Typography.Title level={5}>{t('pages.tunnels.naive.clientUrl')}</Typography.Title>
                <Typography.Paragraph copyable={{ text: clientUrl }}>
                  <Typography.Text code>{clientUrl}</Typography.Text>
                </Typography.Paragraph>
                <Button size="small" onClick={() => setQrOpen(true)}>
                  {t('qrCode')}
                </Button>
              </>
            )}
          </Col>

          <Col xs={24} md={16}>
            <FormProvider {...form}>
              <Form layout="vertical" onFinish={() => void onSave()}>
                <Row gutter={16}>
                  <Col xs={24} sm={12}>
                    <FormField name="remark" control={form.control} label={t('pages.tunnels.naive.form.remark')}>
                      <Input />
                    </FormField>
                  </Col>
                  <Col xs={24} sm={12}>
                    <FormField name="useRawConfig" control={form.control} label={t('pages.tunnels.naive.form.useRawConfig')} valueProp="checked">
                      <Switch />
                    </FormField>
                  </Col>
                </Row>

                {useRaw ? (
                  <>
                    <FormField name="rawConfig" control={form.control} label={t('pages.tunnels.naive.form.rawConfig')}>
                      <Input.TextArea rows={14} spellCheck={false} />
                    </FormField>
                    <Button onClick={() => void onValidateRaw()}>{t('pages.tunnels.naive.form.validate')}</Button>
                  </>
                ) : (
                  <>
                    <Row gutter={16}>
                      <Col xs={24} sm={8}>
                        <FormField name="listen" control={form.control} label={t('pages.tunnels.naive.form.listen')} tooltip={t('pages.tunnels.naive.form.listenTip')}>
                          <Input placeholder="0.0.0.0" />
                        </FormField>
                      </Col>
                      <Col xs={24} sm={8}>
                        <FormField name="port" control={form.control} label={t('port')} required>
                          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                        </FormField>
                      </Col>
                      <Col xs={24} sm={8}>
                        <FormField name="domain" control={form.control} label={t('pages.tunnels.naive.form.domain')} required>
                          <Input placeholder="vpn.example.com" />
                        </FormField>
                      </Col>
                    </Row>

                    <Row gutter={16}>
                      <Col xs={24} sm={8}>
                        <FormField name="useAcme" control={form.control} label={t('pages.tunnels.naive.form.useAcme')} valueProp="checked">
                          <Switch />
                        </FormField>
                      </Col>
                      {useAcme ? (
                        <Col xs={24} sm={16}>
                          <FormField name="acmeEmail" control={form.control} label={t('pages.tunnels.naive.form.acmeEmail')}>
                            <Input placeholder="admin@example.com" />
                          </FormField>
                        </Col>
                      ) : (
                        <>
                          <Col xs={24} sm={8}>
                            <FormField name="certFile" control={form.control} label={t('pages.tunnels.naive.form.certFile')} required>
                              <Input placeholder="/etc/ssl/cert.pem" />
                            </FormField>
                          </Col>
                          <Col xs={24} sm={8}>
                            <FormField name="keyFile" control={form.control} label={t('pages.tunnels.naive.form.keyFile')} required>
                              <Input placeholder="/etc/ssl/key.pem" />
                            </FormField>
                          </Col>
                        </>
                      )}
                    </Row>

                    <Row gutter={16}>
                      <Col xs={24} sm={12}>
                        <FormField name="authUser" control={form.control} label={t('pages.tunnels.naive.form.authUser')} required>
                          <Input autoComplete="off" />
                        </FormField>
                      </Col>
                      <Col xs={24} sm={12}>
                        <FormField
                          name="authPass"
                          control={form.control}
                          label={t('pages.tunnels.naive.form.authPass')}
                          required
                          extra={
                            <Button
                              size="small"
                              type="link"
                              style={{ paddingLeft: 0 }}
                              onClick={() => form.setValue('authPass', generateNaivePassword(), { shouldDirty: true })}
                            >
                              {t('pages.tunnels.naive.form.genPass')}
                            </Button>
                          }
                        >
                          <Input.Password autoComplete="new-password" />
                        </FormField>
                      </Col>
                    </Row>

                    <Row gutter={16}>
                      <Col xs={12} sm={8}>
                        <FormField name="enableH3" control={form.control} label={t('pages.tunnels.naive.form.enableH3')} valueProp="checked">
                          <Switch />
                        </FormField>
                      </Col>
                      <Col xs={12} sm={8}>
                        <FormField name="probeResistance" control={form.control} label={t('pages.tunnels.naive.form.probeResistance')} valueProp="checked">
                          <Switch />
                        </FormField>
                      </Col>
                      <Col xs={12} sm={8}>
                        <FormField name="logLevel" control={form.control} label={t('pages.tunnels.naive.form.logLevel')}>
                          <Select options={['DEBUG', 'INFO', 'WARN', 'ERROR'].map((v) => ({ value: v, label: v }))} />
                        </FormField>
                      </Col>
                    </Row>

                    <Row gutter={16}>
                      <Col xs={24} sm={12}>
                        <FormField
                          name="routeThroughXray"
                          control={form.control}
                          label={t('pages.tunnels.naive.form.routeThroughXray')}
                          tooltip={t('pages.tunnels.naive.form.routeThroughXrayTip')}
                          valueProp="checked"
                        >
                          <Switch />
                        </FormField>
                      </Col>
                      {routeThroughXray ? (
                        <Col xs={24} sm={12}>
                          <FormField
                            name="outboundTag"
                            control={form.control}
                            label={t('pages.tunnels.naive.form.outboundTag')}
                            tooltip={t('pages.tunnels.naive.form.outboundTagTip')}
                          >
                            <Select
                              allowClear
                              showSearch
                              placeholder={t('pages.tunnels.naive.form.outboundTagPlaceholder')}
                              options={(outboundTags ?? []).map((tag) => ({ value: tag, label: tag }))}
                            />
                          </FormField>
                        </Col>
                      ) : null}
                    </Row>

                    <FormField name="extraArgs" control={form.control} label={t('pages.tunnels.naive.form.extraArgs')}>
                      <Input placeholder="--debug" />
                    </FormField>
                  </>
                )}

                <Space>
                  <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={busy}>
                    {t('pages.tunnels.naive.form.save')}
                  </Button>
                  <Button onClick={() => void onPreview()}>{t('pages.tunnels.naive.form.preview')}</Button>
                </Space>
              </Form>
            </FormProvider>
          </Col>
        </Row>
      </Card>

      <Modal
        title={t('pages.tunnels.naive.logsTitle')}
        open={logsOpen}
        footer={null}
        width={720}
        onCancel={() => setLogsOpen(false)}
      >
        <Typography.Paragraph>
          <pre style={{ maxHeight: 420, overflow: 'auto', fontSize: 12 }}>{logsText || t('pages.tunnels.naive.logsEmpty')}</pre>
        </Typography.Paragraph>
      </Modal>

      <Modal
        title={t('pages.tunnels.naive.form.preview')}
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
        title={t('pages.tunnels.naive.binary.download')}
        open={downloadOpen}
        okText={t('pages.tunnels.naive.binary.download')}
        cancelText={t('cancel')}
        confirmLoading={busy}
        onOk={() => void onDownload()}
        onCancel={() => setDownloadOpen(false)}
      >
        <Input
          value={downloadUrl}
          onChange={(e) => setDownloadUrl(e.target.value)}
          placeholder="https://example.com/caddy-naive-linux-amd64"
        />
      </Modal>

      <Modal
        title={`${t('qrCode')} — ${t('pages.tunnels.naive.clientUrl')}`}
        open={qrOpen}
        footer={null}
        onCancel={() => setQrOpen(false)}
      >
        {clientUrl !== '' && <QrPanel value={clientUrl} downloadName="naive-proxy" />}
      </Modal>

      <OlcrtcCard />
      <QwdttCard />
    </div>
  );
}
