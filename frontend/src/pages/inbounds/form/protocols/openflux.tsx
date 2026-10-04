// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Alert, Input } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function OpenfluxFields() {
  const { t } = useTranslation();
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.openfluxNote')}
      />
      <FormField
        name={['settings', 'shareHost']}
        label={t('pages.inbounds.form.openfluxShareHost')}
        tooltip={t('pages.inbounds.form.openfluxShareHostHint')}
      >
        <Input placeholder="203.0.113.5" />
      </FormField>
      <FormField
        name={['settings', 'yandexUrl']}
        label={t('pages.inbounds.form.openfluxYandexUrl')}
        tooltip={t('pages.inbounds.form.openfluxUrlHint')}
      >
        <Input placeholder="https://docs.yandex.ru/docs/view?url=" />
      </FormField>
      <FormField
        name={['settings', 'mailruUrl']}
        label={t('pages.inbounds.form.openfluxMailruUrl')}
        tooltip={t('pages.inbounds.form.openfluxUrlHint')}
      >
        <Input placeholder="https://cloud.mail.ru/public/" />
      </FormField>
      <FormField
        name={['settings', 'cupsUrl']}
        label={t('pages.inbounds.form.openfluxCupsUrl')}
        tooltip={t('pages.inbounds.form.openfluxUrlHint')}
      >
        <Input placeholder="https://interview.cups.online/live-coding/?room=" />
      </FormField>
      <FormField
        name={['settings', 'secret']}
        label={t('pages.inbounds.form.openfluxSecret')}
        tooltip={t('pages.inbounds.form.openfluxSecretHint')}
      >
        <Input.Password autoComplete="new-password" />
      </FormField>
    </>
  );
}
