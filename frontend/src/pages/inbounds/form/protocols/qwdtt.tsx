import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Alert, Switch, Select } from 'antd';
import { useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function QwdttFields() {
  const { t } = useTranslation();
  const routeThroughXray = useWatch({ name: 'settings.routeThroughXray' }) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.qwdttSingleNote')}
      />
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.qwdttRouteThroughXray')}
        tooltip={t('pages.inbounds.form.qwdttRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <FormField
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.qwdttRouteOutbound')}
          tooltip={t('pages.inbounds.form.qwdttRouteOutboundHint')}
        >
          <Select
            showSearch
            optionFilterProp="label"
            options={[
              { value: '', label: t('pages.inbounds.form.qwdttRouteOutboundPlaceholder') },
              ...(outboundTags || []).map((tag) => ({ value: tag, label: tag })),
            ]}
          />
        </FormField>
      )}
      <FormField name={['settings', 'listenAddr']} label={t('pages.inbounds.form.qwdttListenAddr')}>
        <Input placeholder="0.0.0.0:56000" />
      </FormField>
      <FormField name={['settings', 'wgPort']} label={t('pages.inbounds.form.qwdttWgPort')}>
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'password']}
        label={t('pages.inbounds.form.qwdttPassword')}
        tooltip={t('pages.inbounds.form.qwdttPasswordHint')}
      >
        <Input.Password autoComplete="new-password" />
      </FormField>
      <FormField name={['settings', 'dns']} label={t('pages.inbounds.form.qwdttDns')}>
        <Input placeholder="8.8.8.8" />
      </FormField>
      <FormField
        name={['settings', 'subHost']}
        label={t('pages.inbounds.form.qwdttSubHost')}
        tooltip={t('pages.inbounds.form.qwdttSubHostHint')}
      >
        <Input placeholder="1.2.3.4:56000" />
      </FormField>
      <FormField
        name={['settings', 'vkHashes']}
        label={t('pages.inbounds.form.qwdttVkHashes')}
        tooltip={t('pages.inbounds.form.qwdttVkHashesHint')}
      >
        <Input.TextArea rows={2} placeholder="hash1,hash2" />
      </FormField>
      <FormField name={['settings', 'listenRaw']} label={t('pages.inbounds.form.qwdttListenRaw')}>
        <Input placeholder="0.0.0.0:56003" />
      </FormField>
      <FormField
        name={['settings', 'listenDirect']}
        label={t('pages.inbounds.form.qwdttListenDirect')}
      >
        <Input placeholder="" />
      </FormField>
      <FormField name={['settings', 'clientPort']} label={t('pages.inbounds.form.qwdttClientPort')}>
        <InputNumber min={1} max={65535} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'workers']} label={t('pages.inbounds.form.qwdttWorkers')}>
        <InputNumber min={1} max={64} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'configDir']} label={t('pages.inbounds.form.qwdttConfigDir')}>
        <Input placeholder="(default under bin/tunnel/qwdtt-data)" />
      </FormField>
    </>
  );
}
