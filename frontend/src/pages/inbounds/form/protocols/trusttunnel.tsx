// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Button, Collapse, Input, InputNumber, Select, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

function randomHex(bytes: number): string {
  const a = new Uint8Array(bytes);
  crypto.getRandomValues(a);
  return Array.from(a, (b) => b.toString(16).padStart(2, '0')).join('');
}

// Splits the single "ip:port" listen field into address + port inputs so the
// port is visibly selectable (e.g. move TrustTunnel off 443 for the WEB proxy).
function ListenAddrInput({ value, onChange }: { value?: string; onChange?: (v: string) => void }) {
  const raw = (value ?? '').trim() || '0.0.0.0:443';
  const i = raw.lastIndexOf(':');
  const ip = i > 0 ? raw.slice(0, i) : raw;
  const port = i > 0 ? parseInt(raw.slice(i + 1), 10) : 443;
  const safePort = Number.isFinite(port) && port > 0 ? port : 443;
  const emit = (nextIp: string, nextPort: number) =>
    onChange?.(`${nextIp.trim() || '0.0.0.0'}:${nextPort}`);
  return (
    <Space.Compact style={{ width: '100%' }}>
      <Input
        style={{ flex: 1 }}
        value={ip}
        placeholder="0.0.0.0"
        onChange={(e) => emit(e.target.value, safePort)}
      />
      <InputNumber
        style={{ width: 110 }}
        min={1}
        max={65535}
        value={safePort}
        onChange={(v) => emit(ip, typeof v === 'number' && v > 0 ? v : safePort)}
      />
    </Space.Compact>
  );
}

export default function TrustTunnelFields() {
  const { t } = useTranslation();
  const { setValue } = useFormContext();
  const routeThroughXray = useWatch({ name: 'settings.routeThroughXray' }) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();

  const regeneratePrefix = () => {
    const prefix = `${randomHex(4)}/${'f'.repeat(8)}`;
    setValue('settings.clientRandomPrefix', prefix, { shouldDirty: true });
  };

  return (
    <>
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.trustTunnelRouteThroughXray')}
        tooltip={t('pages.inbounds.form.trustTunnelRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <FormField
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.trustTunnelRouteOutbound')}
          tooltip={t('pages.inbounds.form.trustTunnelRouteOutboundHint')}
        >
          <Select
            showSearch
            optionFilterProp="label"
            options={[
              { value: '', label: t('pages.inbounds.form.trustTunnelRouteOutboundPlaceholder') },
              ...(outboundTags || []).map((tag) => ({ value: tag, label: tag })),
            ]}
          />
        </FormField>
      )}
      <FormField
        name={['settings', 'hostname']}
        label={t('pages.inbounds.form.trustTunnelHostname')}
        tooltip={t('pages.inbounds.form.trustTunnelHostnameHint')}
        required
      >
        <Input placeholder="vpn.example.com" />
      </FormField>
      <FormField
        name={['settings', 'listen']}
        label={t('pages.inbounds.form.trustTunnelListen')}
        tooltip={t('pages.inbounds.form.trustTunnelListenHint')}
      >
        <ListenAddrInput />
      </FormField>
      <FormField
        name={['settings', 'certFile']}
        label={t('pages.inbounds.form.trustTunnelCertFile')}
        tooltip={t('pages.inbounds.form.trustTunnelCertFileHint')}
      >
        <Input placeholder="" />
      </FormField>
      <FormField
        name={['settings', 'keyFile']}
        label={t('pages.inbounds.form.trustTunnelKeyFile')}
        tooltip={t('pages.inbounds.form.trustTunnelCertFileHint')}
      >
        <Input placeholder="" />
      </FormField>
      <FormField
        name={['settings', 'upstreamProtocol']}
        label={t('pages.inbounds.form.trustTunnelUpstreamProtocol')}
      >
        <Select
          options={[
            { value: 'http2', label: 'HTTP/2' },
            { value: 'http3', label: 'HTTP/3 (QUIC)' },
          ]}
        />
      </FormField>
      <FormField
        name={['settings', 'clientDns']}
        label={t('pages.inbounds.form.trustTunnelClientDns')}
        tooltip={t('pages.inbounds.form.trustTunnelClientDnsHint')}
      >
        <Input placeholder="1.1.1.1, tls://8.8.8.8" />
      </FormField>
      <FormField
        name={['settings', 'ipv6']}
        label={t('pages.inbounds.form.trustTunnelIpv6')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'listenPreset']}
        label={t('pages.inbounds.form.trustTunnelListenPreset')}
        tooltip={t('pages.inbounds.form.trustTunnelListenPresetHint')}
      >
        <Select
          options={[
            { value: 'fast', label: t('pages.inbounds.form.trustTunnelListenPresetFast') },
            { value: 'stock', label: t('pages.inbounds.form.trustTunnelListenPresetStock') },
          ]}
        />
      </FormField>
      <Collapse
        style={{ marginBottom: 14 }}
        items={[
          {
            key: 'advanced',
            label: t('pages.inbounds.form.trustTunnelAdvanced'),
            children: (
              <Space.Compact style={{ width: '100%' }}>
                <FormField
                  name={['settings', 'clientRandomPrefix']}
                  label={t('pages.inbounds.form.trustTunnelClientRandomPrefix')}
                  tooltip={t('pages.inbounds.form.trustTunnelClientRandomPrefixHint')}
                  style={{ flex: 1, marginBottom: 0 }}
                >
                  <Input readOnly placeholder="aabbccdd/ffffffff" />
                </FormField>
                <Button
                  icon={<ReloadOutlined />}
                  onClick={regeneratePrefix}
                  style={{ marginTop: 30 }}
                  title={t('pages.inbounds.form.trustTunnelClientRandomPrefixRegen')}
                >
                  {t('pages.inbounds.form.trustTunnelClientRandomPrefixRegen')}
                </Button>
              </Space.Compact>
            ),
          },
        ]}
      />
    </>
  );
}
