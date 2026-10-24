// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Alert, Input, InputNumber, Select } from 'antd';
import { useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { WgosocksMaskings } from '@/schemas/protocols/inbound/wgosocks';

export default function WgosocksFields() {
  const { t } = useTranslation();
  const masking = useWatch({ name: 'settings.masking' }) as string | undefined;
  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.wgosocksNote')}
      />
      <FormField
        name={['settings', 'key']}
        label={t('pages.inbounds.form.wgosocksKey')}
        tooltip={t('pages.inbounds.form.wgosocksKeyHint')}
        required
      >
        <Input.Password autoComplete="new-password" placeholder="16 chars, minted on save" />
      </FormField>
      <FormField
        name={['settings', 'masking']}
        label={t('pages.inbounds.form.wgosocksMasking')}
        tooltip={t('pages.inbounds.form.wgosocksMaskingHint')}
        required
      >
        <Select options={WgosocksMaskings.map((m) => ({ value: m, label: m }))} />
      </FormField>
      {masking === 'MEDIA' && (
        <>
          <FormField
            name={['settings', 'mediaSsrc']}
            label={t('pages.inbounds.form.wgoMediaSsrc')}
            tooltip={t('pages.inbounds.form.wgoMediaSsrcHint')}
          >
            <Input placeholder="305419896" />
          </FormField>
          <FormField
            name={['settings', 'mediaPt']}
            label={t('pages.inbounds.form.wgoMediaPt')}
            tooltip={t('pages.inbounds.form.wgoMediaPtHint')}
          >
            <InputNumber min={0} max={127} style={{ width: '100%' }} />
          </FormField>
          <FormField
            name={['settings', 'mediaClock']}
            label={t('pages.inbounds.form.wgoMediaClock')}
            tooltip={t('pages.inbounds.form.wgoMediaClockHint')}
          >
            <InputNumber min={0} max={90000} style={{ width: '100%' }} />
          </FormField>
        </>
      )}
      <FormField
        name={['settings', 'maxDummy']}
        label={t('pages.inbounds.form.wgoMaxDummy')}
        tooltip={t('pages.inbounds.form.wgoMaxDummyHint')}
      >
        <InputNumber min={0} max={1024} style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'idleTimeout']}
        label={t('pages.inbounds.form.wgoIdleTimeout')}
        tooltip={t('pages.inbounds.form.wgoIdleTimeoutHint')}
      >
        <InputNumber min={5} max={86400} style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'maxClients']}
        label={t('pages.inbounds.form.wgosocksMaxClients')}
        tooltip={t('pages.inbounds.form.wgosocksMaxClientsHint')}
      >
        <InputNumber min={1} max={1024} style={{ width: '100%' }} />
      </FormField>
    </>
  );
}
