// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Alert, Input } from 'antd';

import { FormField } from '@/components/form/rhf';

export default function AnytlsFields() {
  const { t } = useTranslation();
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
    </>
  );
}
