// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, Table, Typography, message } from 'antd';
import { useQuery } from '@tanstack/react-query';

import { HttpUtil } from '@/utils';
import { keys } from '@/api/queryKeys';

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
  rows: PreviewRow[];
};

export default function MaskingPage() {
  const { t } = useTranslation();
  const [publicHost, setPublicHost] = useState('');
  const [selected, setSelected] = useState<number[]>([]);
  const [steal, setSteal] = useState<number[]>([]);
  const [busy, setBusy] = useState(false);

  const slimQuery = useQuery({
    queryKey: keys.inbounds.slim(),
    queryFn: async () => {
      const msg = await HttpUtil.get('/panel/api/inbounds/list/slim', undefined, { silent: true });
      if (!msg?.success) throw new Error(msg?.msg || 'list failed');
      return (msg.obj ?? []) as { id: number; protocol: string }[];
    },
  });
  const gateway = (slimQuery.data ?? []).find((ib) => ib.protocol === 'gateway');

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

  const apply = async () => {
    if (!gateway) return;
    setBusy(true);
    try {
      const msg = await HttpUtil.post(`/panel/api/inbounds/${gateway.id}/gatewayApply`, {
        selected,
        steal,
        publicHost,
      });
      if (!msg?.success) throw new Error(msg?.msg);
      void message.success(t('pages.masking.applied'));
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
      const msg = await HttpUtil.post(`/panel/api/inbounds/${gateway.id}/gatewayRevert`, {});
      if (!msg?.success) throw new Error(msg?.msg);
      void message.success(t('pages.masking.reverted'));
      setSelected([]);
      setSteal([]);
      await previewQuery.refetch();
    } catch (e) {
      void message.error(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  };

  if (!gateway) {
    return (
      <Alert
        type="info"
        showIcon
        message={t('pages.masking.noGateway')}
        description={t('pages.masking.createFirst')}
      />
    );
  }

  return (
    <>
      <Typography.Title level={4}>{t('pages.masking.title')}</Typography.Title>
      <Alert type="info" showIcon style={{ marginBottom: 12 }} message={t('pages.masking.help')} />
      <Input
        style={{ maxWidth: 360, marginBottom: 12 }}
        placeholder={t('pages.masking.publicHost')}
        value={publicHost}
        onChange={(e) => setPublicHost(e.target.value)}
      />
      <Table
        rowKey="inboundId"
        size="small"
        dataSource={rows}
        pagination={false}
        rowSelection={{
          selectedRowKeys: selected,
          onChange: (keys) => setSelected(keys.map(Number)),
        }}
        columns={[
          { title: t('remark'), dataIndex: 'remark' },
          { title: t('pages.masking.class'), dataIndex: 'class' },
          { title: 'SNI', dataIndex: 'sni' },
          {
            title: t('pages.masking.listen'),
            render: (_, r: PreviewRow) =>
              `${r.oldListen}:${r.oldPort} → ${r.newListen}:${r.newPort}`,
          },
          {
            title: t('pages.masking.steal'),
            render: (_, r: PreviewRow) =>
              r.stealDest ? (
                <input
                  type="checkbox"
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
      <div style={{ marginTop: 12, display: 'flex', gap: 8 }}>
        <Button
          type="primary"
          loading={busy}
          disabled={applied || selected.length === 0}
          onClick={() => void apply()}
        >
          {t('pages.masking.apply')}
        </Button>
        <Button danger loading={busy} disabled={!applied} onClick={() => void revert()}>
          {t('pages.masking.revert')}
        </Button>
      </div>
    </>
  );
}
