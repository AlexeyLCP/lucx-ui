import { useTranslation } from 'react-i18next';
import { Input, Select, Switch } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function NaiveFields() {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const routeThroughXray = useWatch({ control, name: 'settings.routeThroughXray' }) as boolean | undefined;
  const useAcme = useWatch({ control, name: 'settings.useAcme' }) as boolean | undefined;
  const useRaw = useWatch({ control, name: 'settings.useRawConfig' }) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();

  return (
    <>
      <FormField name={['settings', 'domain']} label={t('pages.inbounds.form.naiveDomain')} tooltip={t('pages.inbounds.form.naiveDomainHint')}>
        <Input placeholder="n.example.com" />
      </FormField>
      <FormField name={['settings', 'useAcme']} label={t('pages.inbounds.form.naiveUseAcme')} tooltip={t('pages.inbounds.form.naiveUseAcmeHint')} valueProp="checked">
        <Switch />
      </FormField>
      {useAcme ? (
        <FormField name={['settings', 'acmeEmail']} label={t('pages.inbounds.form.naiveAcmeEmail')}>
          <Input placeholder="admin@example.com" />
        </FormField>
      ) : (
        <>
          <FormField name={['settings', 'certFile']} label={t('pages.inbounds.form.naiveCertFile')}>
            <Input placeholder="/path/to/fullchain.pem" />
          </FormField>
          <FormField name={['settings', 'keyFile']} label={t('pages.inbounds.form.naiveKeyFile')}>
            <Input placeholder="/path/to/privkey.pem" />
          </FormField>
        </>
      )}
      <FormField name={['settings', 'authUser']} label={t('pages.inbounds.form.naiveAuthUser')} tooltip={t('pages.inbounds.form.naiveAuthServiceHint')}>
        <Input autoComplete="off" />
      </FormField>
      <FormField name={['settings', 'authPass']} label={t('pages.inbounds.form.naiveAuthPass')}>
        <Input.Password autoComplete="new-password" />
      </FormField>
      <FormField name={['settings', 'enableH3']} label={t('pages.inbounds.form.naiveEnableH3')} valueProp="checked">
        <Switch />
      </FormField>
      <FormField name={['settings', 'probeResistance']} label={t('pages.inbounds.form.naiveProbeResistance')} valueProp="checked">
        <Switch />
      </FormField>
      <FormField name={['settings', 'logLevel']} label={t('pages.inbounds.form.naiveLogLevel')}>
        <Select
          options={[
            { value: 'DEBUG', label: 'DEBUG' },
            { value: 'INFO', label: 'INFO' },
            { value: 'WARN', label: 'WARN' },
            { value: 'ERROR', label: 'ERROR' },
          ]}
        />
      </FormField>
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.naiveRouteThroughXray')}
        tooltip={t('pages.inbounds.form.naiveRouteThroughXrayHint')}
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
            allowClear
            showSearch
            options={(outboundTags || []).map((tag) => ({ value: tag, label: tag }))}
            placeholder={t('pages.inbounds.form.naiveRouteOutboundPlaceholder')}
          />
        </FormField>
      )}
      <FormField name={['settings', 'useRawConfig']} label={t('pages.inbounds.form.naiveUseRaw')} tooltip={t('pages.inbounds.form.naiveUseRawHint')} valueProp="checked">
        <Switch />
      </FormField>
      {useRaw && (
        <FormField name={['settings', 'rawConfig']} label={t('pages.inbounds.form.naiveRawConfig')}>
          <Input.TextArea rows={12} placeholder="Caddyfile…" style={{ fontFamily: 'monospace' }} />
        </FormField>
      )}
      <FormField name={['settings', 'extraArgs']} label={t('pages.inbounds.form.naiveExtraArgs')}>
        <Input placeholder="--optional flags" />
      </FormField>
    </>
  );
}
