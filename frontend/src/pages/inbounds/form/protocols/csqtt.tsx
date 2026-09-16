import { useTranslation } from 'react-i18next';
import { Input, Alert } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function CsqttFields() {
  const { t } = useTranslation();
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.csqttSingleNote')}
      />
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
