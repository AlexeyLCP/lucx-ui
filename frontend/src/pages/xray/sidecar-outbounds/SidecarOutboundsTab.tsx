// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useQueryClient } from '@tanstack/react-query';
import { Alert, Button, Popconfirm, Space, Table, Tag, Typography, Upload, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';
import { PlusOutlined, ThunderboltOutlined, UploadOutlined } from '@ant-design/icons';

import { keys } from '@/api/queryKeys';
import { sidecarOutboundsApi } from '@/api/sidecar-outbounds';
import type { SidecarOutbound, SidecarOutboundRow } from '@/schemas/sidecar-outbound';
import { SidecarOutboundFormModal } from './SidecarOutboundFormModal';

export function SidecarOutboundsTab() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const [rows, setRows] = useState<SidecarOutboundRow[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<SidecarOutbound | null>(null);
  const [messageApi, holder] = message.useMessage();

  async function reload() {
    setLoading(true);
    try {
      const res = await sidecarOutboundsApi.list();
      if (res.success) setRows(res.obj ?? []);
    } finally {
      setLoading(false);
    }
  }

  function afterMutate() {
    void reload();
    void queryClient.invalidateQueries({ queryKey: keys.xray.config() });
  }

  useEffect(() => { void reload(); }, []);

  async function handleTest(row: SidecarOutbound) {
    try {
      const res = await sidecarOutboundsApi.test(row.id);
      if (res.success && res.obj?.ok) {
        messageApi.success(t('pages.xray.awgOutbound.testOk'));
      } else {
        messageApi.error(res.obj?.raw || res.msg || t('pages.xray.awgOutbound.test'));
      }
    } catch (e) {
      messageApi.error((e as Error)?.message || 'test failed');
    }
  }

  const columns: ColumnsType<SidecarOutboundRow> = [
    { title: t('pages.xray.sidecarOutbound.protocol'), dataIndex: 'protocol', key: 'protocol' },
    { title: t('pages.xray.awgOutbound.tag'), dataIndex: 'tag', key: 'tag' },
    { title: t('pages.xray.awgOutbound.remark'), dataIndex: 'remark', key: 'remark' },
    {
      title: t('pages.xray.awgOutbound.status'),
      dataIndex: 'status',
      key: 'status',
      render: (s: string, row) => (
        <Space>
          <span>{s}</span>
          {row.binaryMissing ? <Tag color="red">{t('pages.xray.sidecarOutbound.binaryMissing')}</Tag> : null}
        </Space>
      ),
    },
    {
      title: t('pages.xray.awgOutbound.actions'),
      key: 'actions',
      render: (_: unknown, row: SidecarOutboundRow) => (
        <Space wrap>
          {row.protocol === 'naive' && row.binaryMissing ? (
            <Upload
              showUploadList={false}
              beforeUpload={(file) => {
                void sidecarOutboundsApi.upload('naive', file).then(() => {
                  messageApi.success(t('pages.xray.sidecarOutbound.uploaded'));
                  afterMutate();
                }).catch((e: Error) => messageApi.error(e.message));
                return false;
              }}
            >
              <Button size="small" icon={<UploadOutlined />}>
                {t('pages.xray.sidecarOutbound.uploadNaive')}
              </Button>
            </Upload>
          ) : null}
          <Button size="small" icon={<ThunderboltOutlined />} onClick={() => handleTest(row)}>
            {t('pages.xray.awgOutbound.test')}
          </Button>
          <Button size="small" onClick={() => { setEditing(row); setModalOpen(true); }}>{t('edit')}</Button>
          <Button size="small" onClick={() => { void sidecarOutboundsApi.enable(row.id, !row.enable).then(afterMutate); }}>
            {row.enable ? t('disable') : t('enable')}
          </Button>
          <Popconfirm
            title={t('pages.xray.sidecarOutbound.deleteConfirm')}
            okText={t('delete')}
            cancelText={t('cancel')}
            onConfirm={() => { void sidecarOutboundsApi.del(row.id).then(afterMutate); }}
          >
            <Button size="small" danger>{t('delete')}</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ];

  return (
    <>
      {holder}
      <Typography.Title level={5}>{t('pages.xray.sidecarOutbound.sectionTitle')}</Typography.Title>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.xray.sidecarOutbound.hint')}
      />
      <div style={{ marginBottom: 16 }}>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => { setEditing(null); setModalOpen(true); }}
        >
          {t('pages.xray.sidecarOutbound.add')}
        </Button>
      </div>
      <Table columns={columns} dataSource={rows} rowKey="id" loading={loading} />
      <SidecarOutboundFormModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={afterMutate}
        initial={editing}
      />
    </>
  );
}
