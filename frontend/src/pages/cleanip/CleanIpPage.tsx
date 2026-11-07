// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Card,
  Col,
  Form,
  Input,
  InputNumber,
  Row,
  Space,
  Switch,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { CompassOutlined } from '@ant-design/icons';
import { FormProvider, useWatch } from 'react-hook-form';

import { cleanIpApi } from '@/api/cleanip';
import { FormField, useZodForm } from '@/components/form/rhf';
import { CleanIpParamsSchema, type CleanIpResult } from '@/schemas/cleanip';

const DEFAULTS = {
  port: 443,
  count: 30,
  sample: 256,
  timeoutMs: 1500,
  tlsCheck: true,
  sni: 'www.cloudflare.com',
};

export default function CleanIpPage() {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  const [results, setResults] = useState<CleanIpResult[] | undefined>();
  const [scanning, setScanning] = useState(false);

  const form = useZodForm(CleanIpParamsSchema, { defaultValues: DEFAULTS });
  const tlsCheck = useWatch({ control: form.control, name: 'tlsCheck' });

  const onScan = form.handleSubmit(async (values) => {
    setScanning(true);
    try {
      const res = await cleanIpApi.scan(values);
      if (!res.success) {
        messageApi.error(res.msg);
        return;
      }
      setResults(res.obj ?? []);
    } finally {
      setScanning(false);
    }
  });

  const columns = [
    {
      title: t('pages.cleanip.results.address'),
      dataIndex: 'address',
      render: (address: string) => (
        <Typography.Text copyable code style={{ wordBreak: 'break-all' }}>
          {address}
        </Typography.Text>
      ),
    },
    {
      title: t('pages.cleanip.results.latency'),
      dataIndex: 'latencyMs',
      width: 120,
      render: (ms: number) => `${ms} ms`,
    },
    {
      title: t('pages.cleanip.results.tls'),
      dataIndex: 'tlsOk',
      width: 90,
      render: (ok: boolean) => (ok ? <Tag color="green">OK</Tag> : <Tag>n/a</Tag>),
    },
  ];

  return (
    <div style={{ padding: 24 }} data-testid="cleanip-page">
      {messageContextHolder}
      <Typography.Title level={4}>{t('pages.cleanip.title')}</Typography.Title>
      <Typography.Paragraph type="secondary">{t('pages.cleanip.subtitle')}</Typography.Paragraph>
      <Row gutter={16}>
        <Col xs={24} md={9}>
          <Card title={t('pages.cleanip.form.title')}>
            <FormProvider {...form}>
              <Form layout="vertical" onFinish={onScan}>
                <Row gutter={12}>
                  <Col xs={12}>
                    <FormField
                      name="port"
                      control={form.control}
                      label={t('pages.cleanip.form.port')}
                    >
                      <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col xs={12}>
                    <FormField
                      name="count"
                      control={form.control}
                      label={t('pages.cleanip.form.count')}
                    >
                      <InputNumber min={1} max={100} style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col xs={12}>
                    <FormField
                      name="sample"
                      control={form.control}
                      label={t('pages.cleanip.form.sample')}
                    >
                      <InputNumber min={8} max={4096} style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col xs={12}>
                    <FormField
                      name="timeoutMs"
                      control={form.control}
                      label={t('pages.cleanip.form.timeout')}
                    >
                      <InputNumber min={100} max={10000} step={100} style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col xs={24}>
                    <FormField
                      name="tlsCheck"
                      control={form.control}
                      label={t('pages.cleanip.form.tlsCheck')}
                    >
                      <Switch />
                    </FormField>
                  </Col>
                  {tlsCheck && (
                    <Col xs={24}>
                      <FormField
                        name="sni"
                        control={form.control}
                        label={t('pages.cleanip.form.sni')}
                      >
                        <Input />
                      </FormField>
                    </Col>
                  )}
                </Row>
                <Button
                  type="primary"
                  icon={<CompassOutlined aria-hidden />}
                  htmlType="submit"
                  loading={scanning}
                  style={{ marginTop: 8 }}
                >
                  {t('pages.cleanip.form.scan')}
                </Button>
              </Form>
            </FormProvider>
          </Card>
        </Col>
        <Col xs={24} md={15}>
          <Card title={t('pages.cleanip.results.title')}>
            {results && results.length === 0 && (
              <Alert type="info" showIcon message={t('pages.cleanip.results.empty')} />
            )}
            <Space direction="vertical" style={{ width: '100%' }}>
              <Table
                rowKey="address"
                size="small"
                loading={scanning}
                dataSource={results ?? []}
                columns={columns}
                pagination={{ pageSize: 15, hideOnSinglePage: true }}
                locale={{ emptyText: t('pages.cleanip.results.empty') }}
              />
            </Space>
          </Card>
        </Col>
      </Row>
    </div>
  );
}
