// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Input, Alert, Switch, Select } from 'antd';
import { useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function CsqttFields() {
  const { t } = useTranslation();
  const routeThroughXray = useWatch({ name: 'settings.routeThroughXray' }) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.csqttSingleNote')}
      />
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.csqttRouteThroughXray')}
        tooltip={t('pages.inbounds.form.csqttRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <FormField
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.csqttRouteOutbound')}
          tooltip={t('pages.inbounds.form.csqttRouteOutboundHint')}
        >
          <Select
            showSearch
            optionFilterProp="label"
            options={[
              { value: '', label: t('pages.inbounds.form.csqttRouteOutboundPlaceholder') },
              ...(outboundTags || []).map((tag) => ({ value: tag, label: tag })),
            ]}
          />
        </FormField>
      )}
      <FormField name={['settings', 'listenAddr']} label={t('pages.inbounds.form.csqttListenAddr')}>
        <Input placeholder="0.0.0.0:46000" />
      </FormField>
      <FormField
        name={['settings', 'password']}
        label={t('pages.inbounds.form.csqttPassword')}
        tooltip={t('pages.inbounds.form.csqttPasswordHint')}
      >
        <Input.Password autoComplete="new-password" />
      </FormField>
      <FormField
        name={['settings', 'deviceId']}
        label={t('pages.inbounds.form.csqttDeviceId')}
        tooltip={t('pages.inbounds.form.csqttDeviceIdHint')}
      >
        <Input placeholder="device_id" autoComplete="off" />
      </FormField>
      <FormField
        name={['settings', 'subHost']}
        label={t('pages.inbounds.form.csqttSubHost')}
        tooltip={t('pages.inbounds.form.csqttSubHostHint')}
      >
        <Input placeholder="1.2.3.4" />
      </FormField>
      <FormField
        name={['settings', 'vkHashes']}
        label={t('pages.inbounds.form.csqttVkHashes')}
        tooltip={t('pages.inbounds.form.csqttVkHashesHint')}
      >
        <Input.TextArea rows={2} placeholder="hash1,hash2" />
      </FormField>
    </>
  );
}
