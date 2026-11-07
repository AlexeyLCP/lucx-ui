// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useRef } from 'react';
import { useTranslation } from 'react-i18next';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Badge,
  Button,
  Card,
  Col,
  Descriptions,
  Form,
  Input,
  Row,
  Space,
  Switch,
  Typography,
  message,
} from 'antd';
import { ReloadOutlined, SaveOutlined } from '@ant-design/icons';
import { FormProvider } from 'react-hook-form';
import { z } from 'zod';

import { keys } from '@/api/queryKeys';
import { scanguardApi } from '@/api/scanguard';
import { FormField, rhfZodValidate, useZodForm } from '@/components/form/rhf';
import type { ScanguardConfig } from '@/schemas/scanguard';

const toLines = (text: string): string[] =>
  text
    .split(/\r?\n/)
    .map((l) => l.trim())
    .filter(Boolean);

const httpListSchema = z.string().superRefine((value, ctx) => {
  for (const line of toLines(value)) {
    if (!/^https?:\/\//.test(line)) {
      ctx.addIssue({ code: 'custom', message: 'pages.scanguard.validation.httpUrl' });
      return;
    }
  }
});

const FormSchema = z.object({
  enabled: z.boolean(),
  sourcesText: httpListSchema,
  whitelistText: z.string(),
});

const splitLines = (list: string[]): string => list.join('\n');

export default function ScanguardPage() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [messageApi, messageContextHolder] = message.useMessage();

  const { data: statusMsg } = useQuery({
    queryKey: keys.scanguard(),
    queryFn: () => scanguardApi.status(),
    refetchInterval: 15000,
  });
  const status = statusMsg?.success ? (statusMsg.obj ?? undefined) : undefined;

  const form = useZodForm(FormSchema, {
    defaultValues: { enabled: false, sourcesText: '', whitelistText: '' },
  });
  const loadedRef = useRef(false);
  useEffect(() => {
    if (status && !loadedRef.current) {
      form.reset({
        enabled: status.enabled,
        sourcesText: splitLines(status.sources),
        whitelistText: splitLines(status.whitelist),
      });
      loadedRef.current = true;
    }
  }, [status, form]);

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: keys.scanguard() });
  };

  const busy = false;

  const onSave = form.handleSubmit(async (values) => {
    const cfg: ScanguardConfig = {
      enabled: values.enabled,
      sources: toLines(values.sourcesText),
      whitelist: toLines(values.whitelistText),
    };
    const res = await scanguardApi.saveConfig(cfg);
    if (!res.success && res.msg) messageApi.error(res.msg);
    invalidate();
  });

  const onRefresh = async () => {
    const res = await scanguardApi.refresh();
    if (!res.success && res.msg) messageApi.error(res.msg);
    invalidate();
  };

  const firewall = status?.firewall;
  const entries = (firewall?.v4Entries ?? 0) + (firewall?.v6Entries ?? 0);

  return (
    <div style={{ padding: 24 }} data-testid="scanguard-page">
      {messageContextHolder}
      <Typography.Title level={4}>{t('pages.scanguard.title')}</Typography.Title>
      <Typography.Paragraph type="secondary">{t('pages.scanguard.subtitle')}</Typography.Paragraph>
      <Row gutter={16}>
        <Col xs={24} md={10}>
          <Card title={t('pages.scanguard.status.title')} loading={!statusMsg}>
            {!firewall?.ipsetAvailable && (
              <Alert
                type="warning"
                showIcon
                style={{ marginBottom: 12 }}
                message={t('pages.scanguard.status.ipsetMissing')}
              />
            )}
            {status?.lastError && (
              <Alert
                type="error"
                showIcon
                style={{ marginBottom: 12 }}
                message={status.lastError}
              />
            )}
            <Descriptions column={1} size="small">
              <Descriptions.Item label={t('pages.scanguard.status.state')}>
                <Badge
                  status={status?.enabled ? 'success' : 'default'}
                  text={
                    status?.enabled
                      ? t('pages.scanguard.status.enabled')
                      : t('pages.scanguard.status.disabled')
                  }
                />
              </Descriptions.Item>
              <Descriptions.Item label={t('pages.scanguard.status.entries')}>
                {entries}
              </Descriptions.Item>
              <Descriptions.Item label={t('pages.scanguard.status.rules')}>
                {firewall?.rulesV4
                  ? t('pages.scanguard.status.rulesOk')
                  : t('pages.scanguard.status.rulesMissing')}
              </Descriptions.Item>
              <Descriptions.Item label={t('pages.scanguard.status.backend')}>
                {firewall?.backend || '—'}
              </Descriptions.Item>
              <Descriptions.Item label={t('pages.scanguard.status.lastUpdated')}>
                {status?.lastUpdated || t('pages.scanguard.status.never')}
              </Descriptions.Item>
            </Descriptions>
          </Card>
        </Col>
        <Col xs={24} md={14}>
          <Card title={t('pages.scanguard.form.title')}>
            <FormProvider {...form}>
              <Form layout="vertical" onFinish={onSave}>
                <FormField
                  name="enabled"
                  control={form.control}
                  label={t('pages.scanguard.form.enabled')}
                >
                  <Switch />
                </FormField>
                <FormField
                  name="sourcesText"
                  control={form.control}
                  label={t('pages.scanguard.form.sources')}
                  tooltip={t('pages.scanguard.form.sourcesTip')}
                  required
                  rules={{ validate: rhfZodValidate(httpListSchema) }}
                >
                  <Input.TextArea rows={5} placeholder="https://…" />
                </FormField>
                <FormField
                  name="whitelistText"
                  control={form.control}
                  label={t('pages.scanguard.form.whitelist')}
                  tooltip={t('pages.scanguard.form.whitelistTip')}
                >
                  <Input.TextArea rows={3} placeholder="203.0.113.0/24" />
                </FormField>
                <Space style={{ marginTop: 8 }}>
                  <Button
                    type="primary"
                    icon={<SaveOutlined aria-hidden />}
                    htmlType="submit"
                    disabled={busy}
                  >
                    {t('pages.scanguard.form.save')}
                  </Button>
                  <Button
                    icon={<ReloadOutlined aria-hidden />}
                    onClick={() => void onRefresh()}
                    disabled={busy || !status?.enabled}
                  >
                    {t('pages.scanguard.form.refresh')}
                  </Button>
                </Space>
              </Form>
            </FormProvider>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
