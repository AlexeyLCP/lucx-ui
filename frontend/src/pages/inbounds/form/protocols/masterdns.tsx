// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Alert, Input, Select, Switch } from 'antd';
import { useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

const MASTERDNS_ENCRYPTION = [
  { value: 0, label: 'None' },
  { value: 1, label: 'XOR' },
  { value: 2, label: 'ChaCha20' },
  { value: 3, label: 'AES-128-GCM' },
  { value: 4, label: 'AES-192-GCM' },
  { value: 5, label: 'AES-256-GCM' },
] as const;

export default function MasterdnsFields() {
  const { t } = useTranslation();
  const routeThroughXray = useWatch({ name: 'settings.routeThroughXray' }) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.masterdnsSingleNote')}
      />
      <FormField
        name={['settings', 'domains']}
        label={t('pages.inbounds.form.masterdnsDomains')}
        tooltip={t('pages.inbounds.form.masterdnsDomainsHint')}
      >
        <Input placeholder="v.example.com" />
      </FormField>
      <FormField
        name={['settings', 'secret']}
        label={t('pages.inbounds.form.masterdnsSecret')}
        tooltip={t('pages.inbounds.form.masterdnsSecretHint')}
      >
        <Input.Password autoComplete="new-password" />
      </FormField>
      <FormField
        name={['settings', 'encryption']}
        label={t('pages.inbounds.form.masterdnsEncryption')}
        tooltip={t('pages.inbounds.form.masterdnsEncryptionHint')}
      >
        <Select
          options={MASTERDNS_ENCRYPTION.map((o) => ({
            value: o.value,
            label: `${o.value} — ${o.label}`,
          }))}
        />
      </FormField>
      <FormField
        name={['settings', 'resolvers']}
        label={t('pages.inbounds.form.masterdnsResolvers')}
        tooltip={t('pages.inbounds.form.masterdnsResolversHint')}
      >
        <Input placeholder="8.8.8.8:53, 1.1.1.1:53" />
      </FormField>
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.masterdnsRouteThroughXray')}
        tooltip={t('pages.inbounds.form.masterdnsRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <FormField
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.masterdnsRouteOutbound')}
          tooltip={t('pages.inbounds.form.masterdnsRouteOutboundHint')}
        >
          <Select
            showSearch
            optionFilterProp="label"
            options={[
              { value: '', label: t('pages.inbounds.form.masterdnsRouteOutboundPlaceholder') },
              ...(outboundTags || []).map((tag) => ({ value: tag, label: tag })),
            ]}
          />
        </FormField>
      )}
    </>
  );
}
