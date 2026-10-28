// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// LUCX-HOOK: wgobfs obfuscation presets on the kernel WireGuard inbound.
// One engine process (wgo-<id>) renders one INI section per preset; clients
// bound to a preset get a [instance]-carrying .conf / phobos:// export.
import { useTranslation } from 'react-i18next';
import { Alert, Button, Collapse, Input, InputNumber, Select, Space, Switch } from 'antd';
import { useQuery } from '@tanstack/react-query';
import { DeleteOutlined, PlusOutlined, ReloadOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { HttpUtil } from '@/utils';
import { useAwgInboundId } from '@/pages/inbounds/form/awg-inbound-id-context';
import { WgoPresetMaskings, type WgoPreset } from '@/schemas/protocols/inbound/wgo';
import { nextPreset, randomKey } from '@/lib/wgo/preset';

interface WgoEngineStatus {
  running?: boolean;
  listening?: boolean;
  lastLog?: string;
}

function EngineStatusLine({ inboundId }: { inboundId: number | null }) {
  const { t } = useTranslation();
  const statusQuery = useQuery({
    queryKey: ['inbounds', 'wgoStatus', inboundId],
    queryFn: async (): Promise<WgoEngineStatus | null> => {
      if (inboundId == null) return null;
      const response = await HttpUtil.get<WgoEngineStatus>(
        `/panel/api/inbounds/${inboundId}/wgoStatus`,
        undefined,
        { silent: true },
      );
      return response?.success ? (response.obj as WgoEngineStatus) : null;
    },
    enabled: inboundId != null,
  });
  if (inboundId == null) return null;
  const status = statusQuery.data;
  return (
    <Space style={{ marginBottom: 12 }}>
      <Button
        size="small"
        icon={<ReloadOutlined aria-hidden />}
        onClick={() => void statusQuery.refetch()}
      >
        {t('pages.inbounds.form.wgoStatusRefresh')}
      </Button>
      <span>
        {status?.running
          ? t('pages.inbounds.form.wgoStatusRunning')
          : t('pages.inbounds.form.wgoStatusDown')}
        {status?.lastLog ? ` — ${status.lastLog}` : ''}
      </span>
    </Space>
  );
}

export default function WgoObfuscationFields() {
  const { t } = useTranslation();
  const form = useFormContext();
  const inboundId = useAwgInboundId();
  const wgPort = Number(useWatch({ name: 'port' })) || undefined;
  const enabled = useWatch({ name: 'settings.obfuscation.enabled' }) as boolean | undefined;
  const { fields, append, remove } = useFieldArray({ name: 'settings.obfuscation.presets' });
  const presets = (useWatch({ name: 'settings.obfuscation.presets' }) || []) as WgoPreset[];

  const markDefault = (index: number, value: boolean) => {
    presets.forEach((_, i) => {
      form.setValue(`settings.obfuscation.presets.${i}.isDefault`, i === index && value, {
        shouldDirty: true,
      });
    });
    if (value && presets.length > 1) {
      form.setValue(`settings.obfuscation.presets.${index}.isDefault`, true, { shouldDirty: true });
    }
  };

  return (
    <>
      <FormField
        name={['settings', 'obfuscation', 'enabled']}
        label={t('pages.inbounds.form.wgoEnable')}
        tooltip={t('pages.inbounds.form.wgoEnableHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {enabled && (
        <>
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message={t('pages.inbounds.form.wgoHelperWarning')}
          />
          <EngineStatusLine inboundId={inboundId} />
          {fields.map((field, index) => {
            const masking = presets[index]?.masking;
            return (
              <Space key={field.id} direction="vertical" style={{ display: 'flex' }} size={4}>
                <Space.Compact block>
                  <FormField
                    name={['settings', 'obfuscation', 'presets', index, 'name']}
                    label={t('pages.inbounds.form.wgoPresetName')}
                    style={{ marginBottom: 0 }}
                  >
                    <Input placeholder="phobos-ru" />
                  </FormField>
                  <Button
                    aria-label={t('delete')}
                    icon={<DeleteOutlined aria-hidden />}
                    onClick={() => remove(index)}
                    style={{ marginTop: 30 }}
                  />
                </Space.Compact>
                <FormField
                  name={['settings', 'obfuscation', 'presets', index, 'extPort']}
                  label={t('pages.inbounds.form.wgoExtPort')}
                >
                  <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                </FormField>
                <FormField
                  name={['settings', 'obfuscation', 'presets', index, 'key']}
                  label={t('pages.inbounds.form.wgoKey')}
                >
                  <Input.Password
                    autoComplete="new-password"
                    addonAfter={
                      <Button
                        type="text"
                        size="small"
                        icon={<ReloadOutlined aria-hidden />}
                        onClick={() =>
                          form.setValue(`settings.obfuscation.presets.${index}.key`, randomKey(), {
                            shouldDirty: true,
                          })
                        }
                      />
                    }
                  />
                </FormField>
                <FormField
                  name={['settings', 'obfuscation', 'presets', index, 'masking']}
                  label={t('pages.inbounds.form.wgosocksMasking')}
                >
                  <Select options={WgoPresetMaskings.map((m) => ({ value: m, label: m }))} />
                </FormField>
                <Collapse
                  ghost
                  size="small"
                  style={{ margin: 0 }}
                  items={[
                    {
                      key: 'advanced',
                      children: (
                        <>
                          {masking === 'MEDIA' && (
                            <>
                              <FormField
                                name={['settings', 'obfuscation', 'presets', index, 'mediaSsrc']}
                                label={t('pages.inbounds.form.wgoMediaSsrc')}
                              >
                                <Input placeholder="305419896" />
                              </FormField>
                              <FormField
                                name={['settings', 'obfuscation', 'presets', index, 'mediaPt']}
                                label={t('pages.inbounds.form.wgoMediaPt')}
                              >
                                <InputNumber min={0} max={127} style={{ width: '100%' }} />
                              </FormField>
                              <FormField
                                name={['settings', 'obfuscation', 'presets', index, 'mediaClock']}
                                label={t('pages.inbounds.form.wgoMediaClock')}
                              >
                                <InputNumber min={0} max={90000} style={{ width: '100%' }} />
                              </FormField>
                            </>
                          )}
                          <FormField
                            name={['settings', 'obfuscation', 'presets', index, 'maxDummy']}
                            label={t('pages.inbounds.form.wgoMaxDummy')}
                          >
                            <InputNumber min={0} max={1024} style={{ width: '100%' }} />
                          </FormField>
                          <FormField
                            name={['settings', 'obfuscation', 'presets', index, 'idleTimeout']}
                            label={t('pages.inbounds.form.wgoIdleTimeout')}
                          >
                            <InputNumber min={5} max={86400} style={{ width: '100%' }} />
                          </FormField>
                          <FormField
                            name={[
                              'settings',
                              'obfuscation',
                              'presets',
                              index,
                              'clientWgLocalPort',
                            ]}
                            label={t('pages.inbounds.form.wgoClientLocalPort')}
                            tooltip={t('pages.inbounds.form.wgoClientLocalPortHint')}
                          >
                            <InputNumber min={1} max={65535} style={{ width: '100%' }} />
                          </FormField>
                        </>
                      ),
                      label: t('pages.inbounds.form.wgoAdvanced'),
                    },
                  ]}
                />
                <FormField
                  name={['settings', 'obfuscation', 'presets', index, 'isDefault']}
                  label={t('pages.inbounds.form.wgoPresetDefault')}
                  valueProp="checked"
                >
                  <Switch
                    checked={presets[index]?.isDefault}
                    onChange={(v: boolean) => markDefault(index, v)}
                  />
                </FormField>
              </Space>
            );
          })}
          <Button
            block
            icon={<PlusOutlined aria-hidden />}
            onClick={() => append(nextPreset(presets, wgPort))}
          >
            {t('pages.inbounds.form.wgoAddPreset')}
          </Button>
          <FormField
            name={['settings', 'obfuscation', 'blockDirectPort']}
            label={t('pages.inbounds.form.wgoBlockDirect')}
            tooltip={t('pages.inbounds.form.wgoBlockDirectHint')}
            valueProp="checked"
          >
            <Switch />
          </FormField>
        </>
      )}
    </>
  );
}
