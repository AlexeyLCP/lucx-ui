// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Badge,
  Button,
  Card,
  Checkbox,
  Col,
  ConfigProvider,
  Input,
  Layout,
  Popconfirm,
  Row,
  Space,
  Spin,
  Table,
  Tag,
  Typography,
  message,
} from 'antd';
import { useQuery, useQueryClient } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';
import { useTheme } from '@/hooks/useTheme';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import AppSidebar from '@/layouts/AppSidebar';

const JSON_HEADERS = {
  headers: { 'Content-Type': 'application/json' },
} as const;

type PreviewRow = {
  inboundId: number;
  remark: string;
  protocol: string;
  class: string;
  sni: string;
  oldListen: string;
  newListen: string;
  oldPort: number;
  newPort: number;
  stealDest?: string;
};

type PreviewResult = {
  applied: boolean;
  publicHost: string;
  bindIP?: string;
  rows: PreviewRow[];
  ufw?: boolean;
  ufwAllow?: string[];
  hidePanel?: boolean;
};

function classKey(cls: string): string {
  if (cls === 'passthrough') return 'pages.masking.classPassthrough';
  if (cls === 'caddy') return 'pages.masking.classCaddy';
  if (cls === 'skip') return 'pages.masking.classSkip';
  return '';
}

function sniClash(rows: PreviewRow[], chosen: number[]): string {
  const seen = new Set<string>();
  for (const r of rows) {
    if (!chosen.includes(r.inboundId) || r.class === 'skip') continue;
    const sni = r.sni.trim().toLowerCase();
    if (!sni) continue;
    if (seen.has(sni)) return sni;
    seen.add(sni);
  }
  return '';
}

export default function MaskingPage() {
  const { t } = useTranslation();
  const { isDark, isUltra, antdThemeConfig } = useTheme();
  const { isMobile } = useMediaQuery();
  const queryClient = useQueryClient();
  const [publicHost, setPublicHost] = useState('');
  const [selected, setSelected] = useState<number[]>([]);
  const [steal, setSteal] = useState<number[]>([]);
  const [picked, setPicked] = useState(false);
  const [ufw, setUfw] = useState(false);
  const [hidePanel, setHidePanel] = useState(false);
  const [sniEdits, setSniEdits] = useState<Record<number, string>>({});
  const [busy, setBusy] = useState(false);

  const pageClass = useMemo(() => {
    const classes = ['masking-page'];
    if (isDark) classes.push('is-dark');
    if (isUltra) classes.push('is-ultra');
    return classes.join(' ');
  }, [isDark, isUltra]);

  const slimQuery = useQuery({
    queryKey: keys.inbounds.slim(),
    queryFn: async () => {
      const msg = await HttpUtil.get('/panel/api/inbounds/list/slim', undefined, { silent: true });
      if (!msg?.success) throw new Error(msg?.msg || 'list failed');
      return (msg.obj ?? []) as { id: number; protocol: string }[];
    },
  });
  const gateway = (slimQuery.data ?? []).find((ib) => ib.protocol === 'gateway');

  const defaultsQuery = useQuery({
    queryKey: keys.settings.defaults(),
    queryFn: async () => {
      const msg = await HttpUtil.post('/panel/api/setting/defaultSettings', undefined, {
        silent: true,
      });
      if (!msg?.success) throw new Error(msg?.msg || 'defaults failed');
      return (msg.obj ?? {}) as { subDomain?: string; webDomain?: string };
    },
    staleTime: Infinity,
  });
  const settingHost = defaultsQuery.data?.subDomain || defaultsQuery.data?.webDomain || '';

  const previewQuery = useQuery({
    queryKey: ['gatewayPreview', gateway?.id, publicHost],
    queryFn: async () => {
      const msg = await HttpUtil.get(
        `/panel/api/inbounds/${gateway!.id}/gatewayPreview`,
        { publicHost },
        { silent: true },
      );
      if (!msg?.success) throw new Error(msg?.msg || 'preview failed');
      return msg.obj as PreviewResult;
    },
    enabled: Boolean(gateway?.id),
  });
  const preview = previewQuery.data;
  const rows = preview?.rows ?? [];
  const applied = preview?.applied ?? false;
  const bindIP = preview?.bindIP || '';
  const host =
    publicHost || preview?.publicHost || settingHost || rows.find((r) => r.sni)?.sni || '';

  const behind = rows.filter((r) => r.class !== 'skip');
  const outside = rows.filter((r) => r.class === 'skip');
  const chosen = picked ? selected : behind.map((r) => r.inboundId);
  const shown = behind.map((r) => ({
    ...r,
    sni: sniEdits[r.inboundId] ?? r.sni,
  }));
  const clash = sniClash(shown, chosen);
  const coverOn = behind.some((r) => r.protocol === 'cover' && chosen.includes(r.inboundId));
  const httpFront = behind.some(
    (r) => (r.protocol === 'cover' || r.protocol === 'tproxy') && chosen.includes(r.inboundId),
  );
  const ready = slimQuery.isFetched && (!gateway || previewQuery.isFetched);

  const apply = async () => {
    if (!gateway) return;
    setBusy(true);
    try {
      const msg = await HttpUtil.post(
        `/panel/api/inbounds/${gateway.id}/gatewayApply`,
        {
          selected: chosen,
          steal,
          publicHost: host,
          ufw,
          hidePanel,
          sni: Object.fromEntries(
            shown.filter((r) => chosen.includes(r.inboundId)).map((r) => [r.inboundId, r.sni]),
          ),
        },
        JSON_HEADERS,
      );
      if (!msg?.success) throw new Error(msg?.msg);
      void message.success(t('pages.masking.applied'));
      await queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
      await previewQuery.refetch();
    } catch (e) {
      void message.error(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const revert = async () => {
    if (!gateway) return;
    setBusy(true);
    try {
      const msg = await HttpUtil.post(
        `/panel/api/inbounds/${gateway.id}/gatewayRevert`,
        {},
        JSON_HEADERS,
      );
      if (!msg?.success) throw new Error(msg?.msg);
      void message.success(t('pages.masking.reverted'));
      setSelected([]);
      setSteal([]);
      setPicked(false);
      setUfw(false);
      setHidePanel(false);
      setSniEdits({});
      await queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
      await previewQuery.refetch();
    } catch (e) {
      void message.error(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const ensure = async () => {
    setBusy(true);
    try {
      const msg = await HttpUtil.post('/panel/api/inbounds/gatewayEnsure', {}, JSON_HEADERS);
      if (!msg?.success) throw new Error(msg?.msg);
      await queryClient.invalidateQueries({ queryKey: keys.inbounds.root() });
      await slimQuery.refetch();
    } catch (e) {
      void message.error(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  const listenCol = {
    title: t('pages.masking.listen'),
    render: (_: unknown, r: PreviewRow) =>
      `${r.oldListen}:${r.oldPort} → ${r.newListen}:${r.newPort}`,
  };

  const body = !gateway ? (
    <Card hoverable title={t('pages.masking.title')}>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.masking.noGateway')}
        description={t('pages.masking.createFirst')}
      />
      <Button type="primary" loading={busy} onClick={() => void ensure()}>
        {t('pages.masking.enable')}
      </Button>
    </Card>
  ) : (
    <Row gutter={[isMobile ? 8 : 16, isMobile ? 8 : 12]}>
      <Col span={24}>
        <Card
          size="small"
          hoverable
          title={t('pages.masking.title')}
          extra={
            <Space wrap>
              <Badge
                status={applied ? 'success' : 'default'}
                text={
                  applied
                    ? t('pages.masking.statusOn', { ip: bindIP || '0.0.0.0', port: 443 })
                    : undefined
                }
              />
              {preview?.ufw ? <Tag>{t('pages.masking.ufwOn')}</Tag> : null}
              {preview?.hidePanel ? <Tag>{t('pages.masking.hidePanelOn')}</Tag> : null}
            </Space>
          }
        >
          {applied ? null : (
            <Alert
              type="info"
              showIcon
              style={{ marginBottom: 12 }}
              message={t('pages.masking.help')}
            />
          )}
          {!bindIP && !applied ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message={t('pages.masking.noBindIP')}
            />
          ) : null}
          {clash ? (
            <Alert
              type="warning"
              showIcon
              style={{ marginBottom: 12 }}
              message={t('pages.masking.sniClash', { sni: clash })}
            />
          ) : null}
          <Typography.Paragraph type="secondary">
            {t('pages.masking.publicHost')}
          </Typography.Paragraph>
          <Input
            style={{ maxWidth: 360 }}
            placeholder={t('pages.masking.publicHost')}
            value={host}
            onChange={(e) => setPublicHost(e.target.value)}
          />
        </Card>
      </Col>
      <Col span={24}>
        <Card size="small" hoverable title={t('pages.masking.behind443')}>
          <Table
            rowKey="inboundId"
            size="small"
            dataSource={shown}
            pagination={false}
            rowSelection={{
              selectedRowKeys: chosen,
              onChange: (keys) => {
                setPicked(true);
                setSelected(keys.map(Number));
              },
              getCheckboxProps: () => ({ disabled: applied }),
            }}
            columns={[
              { title: t('remark'), dataIndex: 'remark' },
              {
                title: t('pages.masking.class'),
                render: (_: unknown, r: PreviewRow) => t(classKey(r.class) || r.class),
              },
              {
                title: 'SNI',
                render: (_: unknown, r: PreviewRow) => (
                  <Input
                    size="small"
                    value={r.sni}
                    disabled={applied}
                    onChange={(e) =>
                      setSniEdits((cur) => ({ ...cur, [r.inboundId]: e.target.value }))
                    }
                  />
                ),
              },
              listenCol,
              {
                title: t('pages.masking.stealHint'),
                render: (_: unknown, r: PreviewRow) =>
                  r.stealDest && coverOn ? (
                    <Checkbox
                      disabled={applied}
                      checked={steal.includes(r.inboundId)}
                      onChange={(e) => {
                        setSteal((cur) =>
                          e.target.checked
                            ? [...cur, r.inboundId]
                            : cur.filter((id) => id !== r.inboundId),
                        );
                      }}
                    />
                  ) : null,
              },
            ]}
          />
          {outside.length > 0 ? (
            <>
              <Typography.Text strong style={{ display: 'block', margin: '16px 0 8px' }}>
                {t('pages.masking.staysPublic')}
              </Typography.Text>
              <Table
                rowKey="inboundId"
                size="small"
                dataSource={outside}
                pagination={false}
                columns={[
                  { title: t('remark'), dataIndex: 'remark' },
                  {
                    title: t('pages.masking.class'),
                    render: (_: unknown, r: PreviewRow) => t(classKey(r.class) || r.class),
                  },
                  listenCol,
                ]}
              />
            </>
          ) : null}
        </Card>
      </Col>
      <Col span={24}>
        <Card size="small" hoverable>
          <Checkbox
            checked={applied ? Boolean(preview?.ufw) : ufw}
            disabled={applied}
            onChange={(e) => setUfw(e.target.checked)}
          >
            {t('pages.masking.ufw')}
          </Checkbox>
          <Typography.Paragraph type="secondary" style={{ margin: '4px 0 12px' }}>
            {t('pages.masking.ufwHint')}
            {preview?.ufwAllow?.length ? ` ${preview.ufwAllow.join(', ')}` : ''}
          </Typography.Paragraph>
          <Checkbox
            checked={applied ? Boolean(preview?.hidePanel) : hidePanel}
            disabled={applied || !httpFront}
            onChange={(e) => setHidePanel(e.target.checked)}
          >
            {t('pages.masking.hidePanel')}
          </Checkbox>
          <Typography.Paragraph type="secondary" style={{ margin: '4px 0 12px' }}>
            {t('pages.masking.hidePanelHint')}
          </Typography.Paragraph>
          <Space wrap>
            <Popconfirm
              title={t(
                hidePanel && ufw
                  ? 'pages.masking.confirmApplyBoth'
                  : hidePanel
                    ? 'pages.masking.confirmApplyHide'
                    : ufw
                      ? 'pages.masking.confirmApplyUfw'
                      : 'pages.masking.confirmApply',
                {
                  n: chosen.length,
                  ip: bindIP || '0.0.0.0',
                },
              )}
              okText={t('pages.masking.confirmOk')}
              disabled={applied || chosen.length === 0 || Boolean(clash)}
              onConfirm={() => void apply()}
            >
              <Button
                type="primary"
                loading={busy}
                disabled={applied || chosen.length === 0 || Boolean(clash)}
              >
                {t('pages.masking.apply')}
              </Button>
            </Popconfirm>
            <Button danger loading={busy} disabled={!applied} onClick={() => void revert()}>
              {t('pages.masking.revert')}
            </Button>
          </Space>
        </Card>
      </Col>
    </Row>
  );

  return (
    <ConfigProvider theme={antdThemeConfig}>
      <Layout className={pageClass}>
        <AppSidebar />
        <Layout className="content-shell">
          <Layout.Content id="content-layout" className="content-area">
            <Spin spinning={!ready} delay={200} size="large">
              {ready ? body : <div className="loading-spacer" />}
            </Spin>
          </Layout.Content>
        </Layout>
      </Layout>
    </ConfigProvider>
  );
}
