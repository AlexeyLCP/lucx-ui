// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useCallback, useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Modal, Space, Table, Tag, Typography, message } from 'antd';
import type { ColumnsType } from 'antd/es/table';

import { awgImportApi } from '@/api/awg-import';
import type { AwgImportCandidate } from '@/schemas/awg-import';

interface Props {
  openMenu?: number;
  onImported: () => void;
}

export default function AwgImportBanner({ openMenu = 0, onImported }: Props) {
  const { t } = useTranslation();
  const [candidates, setCandidates] = useState<AwgImportCandidate[]>([]);
  const [dismissed, setDismissed] = useState(true);
  const [modalOpen, setModalOpen] = useState(false);
  const [selected, setSelected] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [forceShow, setForceShow] = useState(false);

  const load = useCallback(async () => {
    try {
      const msg = await awgImportApi.preview();
      if (!msg.success || !msg.obj) {
        setCandidates([]);
        return;
      }
      const list = msg.obj.candidates ?? [];
      setCandidates(list);
      setDismissed(Boolean(msg.obj.dismissed));
      setSelected(list.map((c) => c.id));
    } catch {
      setCandidates([]);
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  useEffect(() => {
    if (openMenu > 0) {
      setForceShow(true);
      setModalOpen(true);
      void load();
    }
  }, [openMenu, load]);

  const showBanner = candidates.length > 0 && (!dismissed || forceShow);

  const peerTotal = useMemo(
    () => candidates.reduce((n, c) => n + c.peerCount, 0),
    [candidates],
  );

  const onSkip = async () => {
    await awgImportApi.dismiss();
    setDismissed(true);
    setForceShow(false);
    setModalOpen(false);
  };

  const onCommit = async () => {
    if (selected.length === 0) {
      return;
    }
    setBusy(true);
    try {
      const msg = await awgImportApi.commit(selected);
      if (!msg.success || !msg.obj) {
        message.error(msg.msg || t('pages.inbounds.awgImport.none'));
        return;
      }
      const ok = msg.obj.filter((r) => !r.error);
      const fail = msg.obj.filter((r) => r.error);
      const clients = ok.reduce((n, r) => n + r.clients, 0);
      const missing = ok.reduce((n, r) => n + r.missingKeys, 0);
      if (ok.length > 0) {
        message.success(t('pages.inbounds.awgImport.success', {
          inbounds: ok.length,
          clients,
          missing,
        }));
      }
      if (fail.length > 0) {
        message.warning(fail.map((r) => r.error).join('; '));
      }
      setModalOpen(false);
      setForceShow(false);
      await load();
      onImported();
    } finally {
      setBusy(false);
    }
  };

  const columns: ColumnsType<AwgImportCandidate> = [
    { title: t('pages.inbounds.awgImport.source'), dataIndex: 'source', width: 140 },
    { title: 'if', dataIndex: 'ifname', width: 80 },
    { title: t('pages.inbounds.port'), dataIndex: 'port', width: 80 },
    { title: t('pages.inbounds.awgImport.peers'), dataIndex: 'peerCount', width: 80 },
    {
      title: t('pages.inbounds.awgImport.keys'),
      key: 'keys',
      width: 120,
      render: (_, row) => `${row.keysFound}/${row.peerCount}`,
    },
    {
      title: t('pages.inbounds.awgImport.live'),
      dataIndex: 'live',
      width: 80,
      render: (live: boolean) => (live ? <Tag color="green">up</Tag> : <Tag>down</Tag>),
    },
  ];

  const peerColumns: ColumnsType<AwgImportCandidate['peers'][number]> = [
    { title: 'email', dataIndex: 'email' },
    { title: 'IP', dataIndex: 'allowedIPs', width: 160 },
    {
      title: 'key',
      dataIndex: 'hasKey',
      width: 80,
      render: (has: boolean) => (has ? 'ok' : t('pages.inbounds.awgImport.keysMissing')),
    },
  ];

  if (!showBanner && !modalOpen) {
    return null;
  }

  return (
    <>
      {showBanner && !dismissed && (
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message={t('pages.inbounds.awgImport.bannerTitle')}
          description={t('pages.inbounds.awgImport.bannerBody', { count: candidates.length, peers: peerTotal })}
          action={(
            <Space>
              <Button size="small" type="primary" onClick={() => setModalOpen(true)}>
                {t('pages.inbounds.awgImport.review')}
              </Button>
              <Button size="small" onClick={() => void onSkip()}>
                {t('pages.inbounds.awgImport.skip')}
              </Button>
            </Space>
          )}
        />
      )}
      <Modal
        open={modalOpen}
        title={t('pages.inbounds.awgImport.modalTitle')}
        width={920}
        onCancel={() => setModalOpen(false)}
        footer={[
          <Button key="skip" onClick={() => void onSkip()}>{t('pages.inbounds.awgImport.skip')}</Button>,
          <Button key="ok" type="primary" loading={busy} onClick={() => void onCommit()}>
            {t('pages.inbounds.awgImport.confirm')}
          </Button>,
        ]}
      >
        {candidates.length === 0 ? (
          <Typography.Text>{t('pages.inbounds.awgImport.none')}</Typography.Text>
        ) : (
          <>
            <Typography.Paragraph type="secondary">
              {t('pages.inbounds.awgImport.drop')}
            </Typography.Paragraph>
            <Table<AwgImportCandidate>
              rowKey="id"
              size="small"
              pagination={false}
              dataSource={candidates}
              columns={columns}
              rowSelection={{
                selectedRowKeys: selected,
                onChange: (keys) => setSelected(keys.map(String)),
              }}
              expandable={{
                expandedRowRender: (row) => (
                  <Table
                    rowKey="publicKey"
                    size="small"
                    pagination={row.peers.length > 20 ? { pageSize: 20 } : false}
                    columns={peerColumns}
                    dataSource={row.peers}
                  />
                ),
              }}
            />
          </>
        )}
      </Modal>
    </>
  );
}
