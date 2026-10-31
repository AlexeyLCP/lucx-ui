import { Input, InputNumber, Switch } from 'antd';
import { useTranslation } from 'react-i18next';

import { FormField } from '@/components/form/rhf';

export default function TelemtFields() {
  const { t } = useTranslation();
  return (
    <>
      <FormField
        name={['settings', 'tlsDomain']}
        label={t('pages.inbounds.form.telemtTlsDomain')}
        tooltip={t('pages.inbounds.form.telemtTlsDomainHint')}
      >
        <Input placeholder="www.cloudflare.com" />
      </FormField>
      <FormField
        name="port"
        label={t('pages.inbounds.form.telemtPort')}
        tooltip={t('pages.inbounds.form.telemtPortHint')}
      >
        <InputNumber min={1} max={65535} />
      </FormField>
      <FormField
        name={['settings', 'useMiddleProxy']}
        label={t('pages.inbounds.form.telemtMiddleProxy')}
        tooltip={t('pages.inbounds.form.telemtMiddleProxyHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'fastMode']}
        label={t('pages.inbounds.form.telemtFastMode')}
        tooltip={t('pages.inbounds.form.telemtFastModeHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'mask']}
        label={t('pages.inbounds.form.telemtMask')}
        tooltip={t('pages.inbounds.form.telemtMaskHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}
