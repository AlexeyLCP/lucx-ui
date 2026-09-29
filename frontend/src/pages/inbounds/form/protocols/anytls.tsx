// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Alert, Input, Select, Switch } from 'antd';
import { useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function AnytlsFields() {
  const { t } = useTranslation();
  const routeThroughXray = useWatch({ name: 'settings.routeThroughXray' }) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.anytlsNote')}
      />
      <FormField
        name={['settings', 'sni']}
        label={t('pages.inbounds.form.anytlsSni')}
        tooltip={t('pages.inbounds.form.anytlsSniHint')}
        required
      >
        <Input placeholder="vpn.example.com" />
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
        name={['settings', 'password']}
        label={t('pages.inbounds.form.anytlsPassword')}
        tooltip={t('pages.inbounds.form.anytlsPasswordHint')}
      >
        <Input.Password autoComplete="new-password" />
      </FormField>
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.anytlsRouteThroughXray')}
        tooltip={t('pages.inbounds.form.anytlsRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <FormField
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.naiveRouteOutbound')}
          tooltip={t('pages.inbounds.form.naiveRouteOutboundHint')}
        >
          <Select
            showSearch
            optionFilterProp="label"
            options={[
              { value: '', label: t('pages.inbounds.form.naiveRouteOutboundPlaceholder') },
              ...(outboundTags || []).map((tag) => ({ value: tag, label: tag })),
            ]}
          />
        </FormField>
      )}
    </>
  );
}
