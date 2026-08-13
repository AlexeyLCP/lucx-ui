// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Input, Select, Switch } from 'antd';
import { useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function TrustTunnelFields() {
  const { t } = useTranslation();
  const routeThroughXray = useWatch({ name: 'settings.routeThroughXray' }) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();
  return (
    <>
      <FormField
        name={['settings', 'hostname']}
        label={t('pages.inbounds.form.trustTunnelHostname')}
        tooltip={t('pages.inbounds.form.trustTunnelHostnameHint')}
        required
      >
        <Input placeholder="vpn.example.com" />
      </FormField>
      <FormField name={['settings', 'listen']} label={t('pages.inbounds.form.trustTunnelListen')}>
        <Input placeholder="0.0.0.0:443" />
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
            allowClear
            showSearch
            options={(outboundTags || []).map((tag) => ({ value: tag, label: tag }))}
            placeholder={t('pages.inbounds.form.trustTunnelRouteOutboundPlaceholder')}
          />
        </FormField>
      )}
    </>
  );
}
