// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useTranslation } from 'react-i18next';
import { Button, Col, Input, InputNumber, Row, Select, Switch } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

function BindingPortInput({ index }: { index: number }) {
  const { t } = useTranslation();
  const { setValue } = useFormContext();
  const port = useWatch({ name: `settings.portBindings.${index}.port` }) as number | undefined;
  const portRange = useWatch({ name: `settings.portBindings.${index}.portRange` }) as string | undefined;
  const value = portRange ?? (port ? String(port) : '');
  return (
    <Input
      placeholder={t('pages.inbounds.form.mieruPortPlaceholder')}
      value={value}
      onChange={(e) => {
        const v = e.target.value.trim();
        if (v.includes('-')) {
          setValue(`settings.portBindings.${index}.portRange`, v, { shouldDirty: true });
          setValue(`settings.portBindings.${index}.port`, undefined, { shouldDirty: true });
        } else {
          const n = v === '' ? undefined : Number(v);
          setValue(`settings.portBindings.${index}.port`, Number.isFinite(n) ? n : undefined, { shouldDirty: true });
          setValue(`settings.portBindings.${index}.portRange`, undefined, { shouldDirty: true });
        }
      }}
    />
  );
}

export default function MieruFields() {
  const { t } = useTranslation();
  const { control } = useFormContext();
  const routeThroughXray = useWatch({ name: 'settings.routeThroughXray' }) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();
  const { fields, append, remove } = useFieldArray({ control, name: 'settings.portBindings' });

  return (
    <>
      <div style={{ marginBottom: 8 }}>{t('pages.inbounds.form.mieruBindingsHint')}</div>
      {fields.map((f, i) => (
        <Row gutter={8} key={f.id} align="middle" style={{ marginBottom: 8 }}>
          <Col flex="1">
            <BindingPortInput index={i} />
          </Col>
          <Col style={{ width: 110 }}>
            <FormField name={['settings', 'portBindings', i, 'protocol']} noStyle>
              <Select
                options={[
                  { value: 'TCP', label: 'TCP' },
                  { value: 'UDP', label: 'UDP' },
                ]}
              />
            </FormField>
          </Col>
          <Col>
            <Button icon={<DeleteOutlined />} danger onClick={() => remove(i)} disabled={fields.length <= 1} />
          </Col>
        </Row>
      ))}
      <Button
        icon={<PlusOutlined />}
        onClick={() => append({ port: 20101, protocol: 'TCP' })}
        style={{ marginBottom: 16 }}
      >
        {t('pages.inbounds.form.mieruAddBinding')}
      </Button>
      <FormField name={['settings', 'mtu']} label={t('pages.inbounds.form.mieruMtu')}>
        <InputNumber min={1280} max={1500} style={{ width: '100%' }} />
      </FormField>
      <FormField name={['settings', 'loggingLevel']} label={t('pages.inbounds.form.mieruLoggingLevel')}>
        <Select
          options={['DEBUG', 'INFO', 'WARN', 'ERROR'].map((l) => ({ value: l, label: l }))}
        />
      </FormField>
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.mieruRouteThroughXray')}
        tooltip={t('pages.inbounds.form.mieruRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <FormField
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.mieruRouteOutbound')}
          tooltip={t('pages.inbounds.form.mieruRouteOutboundHint')}
        >
          <Select
            allowClear
            showSearch
            options={(outboundTags || []).map((tag) => ({ value: tag, label: tag }))}
            placeholder={t('pages.inbounds.form.mieruRouteOutboundPlaceholder')}
          />
        </FormField>
      )}
    </>
  );
}
