// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Col, Collapse, Input, InputNumber, Row, Select, Switch } from 'antd';
import { DeleteOutlined, PlusOutlined } from '@ant-design/icons';
import { useFieldArray, useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';
import { mieruPreset, type MieruPresetName } from '@/lib/mieru/presets';
import {
  MIERU_HANDSHAKE_MODES,
  MIERU_LOW_ENTROPY_MODES,
  MIERU_MASK_ROTATIONS,
  MIERU_MULTIPLEXING_LEVELS,
  MIERU_NONCE_TYPES,
} from '@/schemas/protocols/inbound/mieru';

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

function MieruAdvancedFields() {
  const { t } = useTranslation();
  const { setValue } = useFormContext();
  const [preset, setPreset] = useState<MieruPresetName>('off');

  const applyPreset = () => {
    const v = mieruPreset(preset);
    setValue('settings.multiplexing', v.multiplexing, { shouldDirty: true });
    setValue('settings.handshakeMode', v.handshakeMode, { shouldDirty: true });
    setValue('settings.trafficPattern', v.trafficPattern, { shouldDirty: true });
  };

  return (
    <>
      <Row gutter={8} align="middle" style={{ marginBottom: 12 }}>
        <Col flex="1">
          <Select
            value={preset}
            onChange={(v) => setPreset(v)}
            options={[
              { value: 'off', label: t('pages.inbounds.form.mieruPresetOff') },
              { value: 'lite', label: t('pages.inbounds.form.mieruPresetLite') },
              { value: 'standard', label: t('pages.inbounds.form.mieruPresetStandard') },
              { value: 'stealth', label: t('pages.inbounds.form.mieruPresetStealth') },
            ]}
          />
        </Col>
        <Col>
          <Button onClick={applyPreset}>{t('pages.inbounds.form.mieruPresetApply')}</Button>
        </Col>
      </Row>
      <FormField
        name={['settings', 'multiplexing']}
        label={t('pages.inbounds.form.mieruMultiplexing')}
        tooltip={t('pages.inbounds.form.mieruMultiplexingHint')}
      >
        <Select
          allowClear
          options={MIERU_MULTIPLEXING_LEVELS.map((v) => ({ value: v, label: v.replace('MULTIPLEXING_', '') }))}
        />
      </FormField>
      <FormField
        name={['settings', 'handshakeMode']}
        label={t('pages.inbounds.form.mieruHandshakeMode')}
        tooltip={t('pages.inbounds.form.mieruHandshakeModeHint')}
      >
        <Select
          allowClear
          options={MIERU_HANDSHAKE_MODES.map((v) => ({ value: v, label: v.replace('HANDSHAKE_', '') }))}
        />
      </FormField>
      <div style={{ margin: '4px 0 8px', fontWeight: 500 }}>{t('pages.inbounds.form.mieruTrafficPattern')}</div>
      <div style={{ marginBottom: 8 }}>{t('pages.inbounds.form.mieruTrafficPatternHint')}</div>
      <FormField
        name={['settings', 'trafficPattern', 'seed']}
        label={t('pages.inbounds.form.mieruTpSeed')}
        tooltip={t('pages.inbounds.form.mieruTpSeedHint')}
      >
        <InputNumber min={0} max={2147483647} style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'trafficPattern', 'unlockAll']}
        label={t('pages.inbounds.form.mieruTpUnlockAll')}
        tooltip={t('pages.inbounds.form.mieruTpUnlockAllHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField
        name={['settings', 'trafficPattern', 'tcpFragment', 'enable']}
        label={t('pages.inbounds.form.mieruTpTcpFragmentEnable')}
        tooltip={t('pages.inbounds.form.mieruTpTcpFragmentEnableHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <FormField name={['settings', 'trafficPattern', 'tcpFragment', 'maxSleepMs']} label={t('pages.inbounds.form.mieruTpMaxSleepMs')}>
        <InputNumber min={0} max={100} style={{ width: '100%' }} />
      </FormField>
      <FormField
        name={['settings', 'trafficPattern', 'nonce', 'type']}
        label={t('pages.inbounds.form.mieruTpNonceType')}
        tooltip={t('pages.inbounds.form.mieruTpNonceTypeHint')}
      >
        <Select allowClear options={MIERU_NONCE_TYPES.map((v) => ({ value: v, label: v.replace('NONCE_TYPE_', '') }))} />
      </FormField>
      <FormField
        name={['settings', 'trafficPattern', 'nonce', 'applyToAllUDPPacket']}
        label={t('pages.inbounds.form.mieruTpNonceAllUdp')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      <Row gutter={8}>
        <Col span={12}>
          <FormField name={['settings', 'trafficPattern', 'nonce', 'minLen']} label={t('pages.inbounds.form.mieruTpNonceMinLen')}>
            <InputNumber min={0} max={12} style={{ width: '100%' }} />
          </FormField>
        </Col>
        <Col span={12}>
          <FormField name={['settings', 'trafficPattern', 'nonce', 'maxLen']} label={t('pages.inbounds.form.mieruTpNonceMaxLen')}>
            <InputNumber min={0} max={12} style={{ width: '100%' }} />
          </FormField>
        </Col>
      </Row>
      <FormField
        name={['settings', 'trafficPattern', 'nonce', 'customHexStrings']}
        label={t('pages.inbounds.form.mieruTpNonceHex')}
        tooltip={t('pages.inbounds.form.mieruTpNonceHexHint')}
      >
        <Select mode="tags" tokenSeparators={[',', ' ']} placeholder="00010203" />
      </FormField>
      <Row gutter={8}>
        <Col span={12}>
          <FormField
            name={['settings', 'trafficPattern', 'padding', 'maxMiddlePaddingLen']}
            label={t('pages.inbounds.form.mieruTpPaddingMiddle')}
            tooltip={t('pages.inbounds.form.mieruTpPaddingHint')}
          >
            <InputNumber min={0} max={255} style={{ width: '100%' }} />
          </FormField>
        </Col>
        <Col span={12}>
          <FormField
            name={['settings', 'trafficPattern', 'padding', 'maxEndPaddingLen']}
            label={t('pages.inbounds.form.mieruTpPaddingEnd')}
            tooltip={t('pages.inbounds.form.mieruTpPaddingHint')}
          >
            <InputNumber min={0} max={255} style={{ width: '100%' }} />
          </FormField>
        </Col>
      </Row>
      <FormField
        name={['settings', 'trafficPattern', 'lowEntropy', 'mode']}
        label={t('pages.inbounds.form.mieruTpLowEntropyMode')}
        tooltip={t('pages.inbounds.form.mieruTpLowEntropyModeHint')}
      >
        <Select
          allowClear
          options={MIERU_LOW_ENTROPY_MODES.map((v) => ({ value: v, label: v.replace('LOW_ENTROPY_MODE_', '') }))}
        />
      </FormField>
      <FormField name={['settings', 'trafficPattern', 'lowEntropy', 'maskRotation']} label={t('pages.inbounds.form.mieruTpMaskRotation')}>
        <Select
          allowClear
          showSearch
          options={MIERU_MASK_ROTATIONS.map((v) => ({
            value: v,
            label: v.replace('LOW_ENTROPY_MASK_', '').replace('ROTATE_', ''),
          }))}
        />
      </FormField>
    </>
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
      <Collapse
        style={{ marginBottom: 14 }}
        items={[
          {
            key: 'advanced',
            label: t('pages.inbounds.form.mieruAdvanced'),
            children: <MieruAdvancedFields />,
          },
        ]}
      />
    </>
  );
}
