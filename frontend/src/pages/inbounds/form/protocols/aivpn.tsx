// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Alert,
  Button,
  Input,
  InputNumber,
  Modal,
  Popconfirm,
  Space,
  Switch,
  Table,
  Typography,
  message,
} from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { keys } from '@/api/queryKeys';
import { tunnelsApi } from '@/api/tunnels';
import { FormField } from '@/components/form/rhf';
import { SizeFormatter } from '@/utils';
import { QrPanel } from '@/pages/inbounds/qr';
import { useAwgInboundId } from '@/pages/inbounds/form/awg-inbound-id-context';
import type { AivpnClient } from '@/schemas/protocols/inbound/aivpn';

// AIVPN (infosave2007/aivpn) inbound form. The port/UDP column of the
// inbound row is the mask gateway port; one server per host. The client
// credentials live in the core (clients.json) and are minted server-side —
// the panel only relays the management API and shows the issued `aivpn://`
// key (Rule 0: nothing is generated or rotated here).
export default function AivpnFields() {
  const { t } = useTranslation();
  const inboundId = useAwgInboundId();
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.aivpnSingleNote')}
      />
      <FormField name={['settings', 'listenHost']} label={t('pages.inbounds.form.aivpnListen')}>
        <Input placeholder="0.0.0.0" />
      </FormField>
      <FormField
        name={['settings', 'subnet']}
        label={t('pages.inbounds.form.aivpnSubnet')}
        tooltip={t('pages.inbounds.form.aivpnSubnetHint')}
      >
        <Input placeholder="10.200.0.1/24" />
      </FormField>
      <FormField
        name={['settings', 'serverIp']}
        label={t('pages.inbounds.form.aivpnServerIp')}
        tooltip={t('pages.inbounds.form.aivpnServerIpHint')}
      >
        <Input placeholder="vpn.example.com:45001" />
      </FormField>
      <FormField
        name={['settings', 'mtu']}
        label={t('pages.inbounds.form.aivpnMtu')}
        tooltip={t('pages.inbounds.form.aivpnMtuHint')}
      >
        <InputNumber min={0} max={1500} style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'ipv6']}
        label={t('pages.inbounds.form.aivpnIpv6')}
        tooltip={t('pages.inbounds.form.aivpnIpv6Hint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <AivpnClients inboundId={inboundId} />
    </>
  );
}

function AivpnClients({ inboundId }: { inboundId: number | null }) {
  const { t } = useTranslation();
  const messageApi = message.useMessage()[0];
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [qr, setQr] = useState<{ client: AivpnClient; key: string } | null>(null);

  const invalidate = () => {
    if (inboundId) {
      void queryClient.invalidateQueries({ queryKey: keys.tunnels.aivpnClients(inboundId) });
    }
  };

  const { data, isLoading, refetch } = useQuery({
    queryKey: inboundId ? keys.tunnels.aivpnClients(inboundId) : ['tunnels', 'aivpn-clients-off'],
    queryFn: () => tunnelsApi.aivpnClients(inboundId ?? 0),
    refetchInterval: 10000,
    enabled: !!inboundId,
  });

  const addClient = useMutation({
    mutationFn: (n: string) => tunnelsApi.aivpnAddClient(inboundId ?? 0, n),
    onSuccess: (res) => {
      if (res.success) {
        setName('');
        messageApi.success(t('pages.inbounds.form.aivpn.toasts.clientAdded'));
        invalidate();
      } else if (res.msg) {
        messageApi.error(res.msg);
      }
    },
  });
  const fetchKey = useMutation({
    // v1.2.0 strips the key from every payload; the modal pulls it on an
    // explicit operator click (the core re-derives it, Rule 0 reprint only).
    mutationFn: (row: AivpnClient) =>
      tunnelsApi.aivpnClientKey(inboundId ?? 0, row.id).then((res) => ({ row, res })),
    onSuccess: ({ row, res }) => {
      if (res.success && res.obj) {
        setQr({ client: row, key: res.obj });
      } else if (res.msg) {
        messageApi.error(res.msg);
      }
    },
  });
  const toggleClient = useMutation({
    mutationFn: (row: { id: string; enabled: boolean }) =>
      tunnelsApi.aivpnEnableClient(inboundId ?? 0, row.id, row.enabled),
    onSuccess: () => invalidate(),
  });
  const deleteClient = useMutation({
    mutationFn: (id: string) => tunnelsApi.aivpnDeleteClient(inboundId ?? 0, id),
    onSuccess: () => invalidate(),
  });

  if (!inboundId) {
    return (
      <Typography.Paragraph type="secondary" style={{ marginTop: 16 }}>
        {t('pages.inbounds.form.aivpnSaveFirst')}
      </Typography.Paragraph>
    );
  }

  const rows = (Array.isArray(data?.obj) ? data?.obj : []) as AivpnClient[];
  return (
    <div style={{ marginTop: 16 }}>
      <Typography.Title level={5}>{t('pages.inbounds.form.aivpnClients')}</Typography.Title>
      <Space.Compact style={{ width: '100%', marginBottom: 12 }}>
        <Input
          value={name}
          onChange={(e) => setName(e.target.value)}
          placeholder={t('pages.inbounds.form.aivpnClientName')}
          onPressEnter={() => name.trim() && addClient.mutate(name.trim())}
        />
        <Button
          type="primary"
          loading={addClient.isPending}
          disabled={!name.trim()}
          onClick={() => addClient.mutate(name.trim())}
        >
          {t('add')}
        </Button>
        <Button icon={<ReloadOutlined aria-hidden />} onClick={() => void refetch()} />
      </Space.Compact>
      <Table
        size="small"
        rowKey={(r) => r.id || r.name}
        loading={isLoading}
        dataSource={rows}
        pagination={false}
        columns={[
          {
            title: t('pages.inbounds.form.aivpnClientName'),
            dataIndex: 'name',
            width: 220,
          },
          {
            title: t('pages.inbounds.form.aivpnClientEnabled'),
            dataIndex: 'enabled',
            width: 90,
            render: (_: unknown, row: AivpnClient) => (
              <Switch
                size="small"
                checked={row.enabled}
                onChange={(v) => toggleClient.mutate({ id: row.id, enabled: v })}
              />
            ),
          },
          {
            title: t('pages.inbounds.form.aivpnClientDownload'),
            dataIndex: 'bytesIn',
            width: 110,
            render: (v: number) => SizeFormatter.sizeFormat(v),
          },
          {
            title: t('pages.inbounds.form.aivpnClientUpload'),
            dataIndex: 'bytesOut',
            width: 110,
            render: (v: number) => SizeFormatter.sizeFormat(v),
          },
          {
            title: t('pages.inbounds.form.aivpnClientSeen'),
            dataIndex: 'lastConnected',
            width: 170,
            render: (v: number) =>
              v > 0
                ? new Date(v * 1000).toLocaleString()
                : t('pages.inbounds.form.aivpnClientNever'),
          },
          {
            title: '',
            width: 180,
            render: (_: unknown, row: AivpnClient) => (
              <Space>
                <Button
                  size="small"
                  loading={fetchKey.isPending && fetchKey.variables?.id === row.id}
                  onClick={() => fetchKey.mutate(row)}
                >
                  {t('qrCode')}
                </Button>
                <Popconfirm
                  title={t('pages.inbounds.form.aivpnClientDeleteConfirm')}
                  okText={t('delete')}
                  cancelText={t('cancel')}
                  onConfirm={() => deleteClient.mutate(row.id)}
                >
                  <Button size="small" danger type="text">
                    {t('delete')}
                  </Button>
                </Popconfirm>
              </Space>
            ),
          },
        ]}
      />
      <Modal
        title={`${t('qrCode')} — AIVPN`}
        open={!!qr}
        footer={null}
        width={420}
        onCancel={() => setQr(null)}
      >
        {qr?.key && <QrPanel value={qr.key} remark={qr.client.name} downloadName="aivpn" />}
      </Modal>
    </div>
  );
}
