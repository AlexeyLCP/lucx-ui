// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Popconfirm, Space, Table, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PlusOutlined, ThunderboltOutlined } from '@ant-design/icons';

import { awgOutboundsApi } from '@/api/awg-outbounds';
import type { AwgOutbound, AwgOutboundRow } from '@/schemas/awg-outbound';
import { AwgOutboundStatusBadge, parseAwgOutboundStatus } from './AwgOutboundStatusBadge';
import { AwgOutboundFormModal } from './AwgOutboundFormModal';

export function AwgOutboundsTab() {
  const { t } = useTranslation();
  const [rows, setRows] = useState<AwgOutboundRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<AwgOutbound | null>(null);
  const [messageApi, messageContextHolder] = message.useMessage();

  async function reload() {
    setLoading(true);
    try {
      const res = await awgOutboundsApi.list();
      if (res.success) setRows(res.obj ?? []);
    } finally {
      setLoading(false);
    }
  }

  useEffect(() => { void reload(); }, []);

  async function handleTest(row: AwgOutbound) {
    try {
      const res = await awgOutboundsApi.test(row.id);
      if (res.success && res.obj?.ok) {
        messageApi.success(`${t('pages.xray.awgOutbound.testOk')} (${res.obj.latency_ms} ms)`);
      } else {
        messageApi.error(res.obj?.raw || res.msg || t('pages.xray.awgOutbound.test'));
      }
    } catch (e) {
      messageApi.error((e as Error)?.message || 'test failed');
    }
  }

  async function handleEnable(row: AwgOutbound, enable: boolean) {
    try {
      await awgOutboundsApi.enable(row.id, enable);
      await reload();
    } catch (e) {
      messageApi.error((e as Error)?.message || 'failed');
    }
  }

  async function handleDel(row: AwgOutbound) {
    try {
      await awgOutboundsApi.del(row.id);
      await reload();
    } catch (e) {
      messageApi.error((e as Error)?.message || 'failed');
    }
  }

  const columns: ColumnsType<AwgOutboundRow> = [
    { title: t('pages.xray.awgOutbound.tag'), dataIndex: 'tag', key: 'tag' },
    { title: t('pages.xray.awgOutbound.remark'), dataIndex: 'remark', key: 'remark' },
    {
      title: t('pages.xray.awgOutbound.enable'),
      dataIndex: 'enable',
      key: 'enable',
      render: (enable: boolean) => (enable ? t('enable') : t('disable')),
    },
    {
      title: t('pages.xray.awgOutbound.status'),
      dataIndex: 'status',
      key: 'status',
      render: (s: string) => <AwgOutboundStatusBadge status={parseAwgOutboundStatus(s)} />,
    },
    {
      title: t('pages.xray.awgOutbound.actions'),
      key: 'actions',
      render: (_: unknown, row: AwgOutboundRow) => (
        <Space>
          <Button
            size="small"
            icon={<ThunderboltOutlined />}
            onClick={() => handleTest(row)}
          >
            {t('pages.xray.awgOutbound.test')}
          </Button>
          <Button
            size="small"
            onClick={() => { setEditing(row); setModalOpen(true); }}
          >
            {t('edit')}
          </Button>
          <Button size="small" onClick={() => handleEnable(row, !row.enable)}>
            {row.enable ? t('disable') : t('enable')}
          </Button>
          <Popconfirm
            title={t('pages.xray.awgOutbound.deleteConfirm')}
            okText={t('delete')}
            cancelText={t('cancel')}
            onConfirm={() => handleDel(row)}
          >
            <Button size="small" danger>{t('delete')}</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <>
      {messageContextHolder}
      <div style={{ marginBottom: 16 }}>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => { setEditing(null); setModalOpen(true); }}
        >
          {t('pages.xray.awgOutbound.add')}
        </Button>
      </div>
      <Table columns={columns} dataSource={rows} rowKey="id" loading={loading} />
      <AwgOutboundFormModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={() => void reload()}
        initial={editing}
      />
    </>
  );
}