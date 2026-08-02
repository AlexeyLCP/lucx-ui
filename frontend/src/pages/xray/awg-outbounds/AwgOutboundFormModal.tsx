// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import {
  Alert,
  Button,
  Col,
  Drawer,
  Form,
  Input,
  InputNumber,
  Modal,
  Row,
  Select,
  Space,
  Switch,
  message,
} from 'antd';
import { FormProvider, useForm } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { awgOutboundsApi } from '@/api/awg-outbounds';
import type { AwgOutbound, AwgOutboundSettings } from '@/schemas/awg-outbound';
import type { AwgVersion } from '@/lib/xray/inbound-link';
import { Wireguard } from '@/utils';

interface Props {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  initial?: AwgOutbound | null;
}

interface AwgOutboundFormValues {
  tag: string;
  remark: string;
  enable: boolean;
  // Settings fields, flattened for the form (re-packed into a JSON-string
  // settings object on submit, re-flattened on edit-load).
  privateKey: string;
  address: string;
  mtu: number;
  publicKey: string;
  psk: string;
  endpoint: string;
  keepalive: number;
  allowedIPs: string;
  dns: string;
  jc: number;
  jmin: number;
  jmax: number;
  s1: number;
  s2: number;
  s3: number;
  s4: number;
  h1: string;
  h2: string;
  h3: string;
  h4: string;
  i1: string;
  i2: string;
  i3: string;
  i4: string;
  i5: string;
  headerProtectionKey: string;
  awgVersion: AwgVersion;
  contentPaddingAddition: number;
  rekeyAfterTime: number;
  rekeyTimeout: number;
  rejectAfterTime: number;
  keepaliveTimeout: number;
  maxHandshakeAttempts: number;
  advancedSecurity: boolean;
}

const DEFAULT_SETTINGS: AwgOutboundSettings = {
  privateKey: '',
  address: '',
  mtu: 1320,
  publicKey: '',
  psk: '',
  endpoint: '',
  keepalive: 25,
  allowedIPs: '0.0.0.0/0, ::/0',
  dns: '',
  jc: 0,
  jmin: 0,
  jmax: 0,
  s1: 0,
  s2: 0,
  s3: 0,
  s4: 0,
  h1: '',
  h2: '',
  h3: '',
  h4: '',
  i1: '',
  i2: '',
  i3: '',
  i4: '',
  i5: '',
  headerProtectionKey: '',
  awgVersion: '2',
  contentPaddingAddition: 0,
  rekeyAfterTime: 0,
  rekeyTimeout: 0,
  rejectAfterTime: 0,
  keepaliveTimeout: 0,
  maxHandshakeAttempts: 0,
  advancedSecurity: false,
};

function buildDefaultValues(): AwgOutboundFormValues {
  return {
    tag: '',
    remark: '',
    enable: true,
    ...DEFAULT_SETTINGS,
  };
}

// settingsToFormValues parses the JSON-string settings payload stored on the
// wire (mirrors Inbound.Settings) and flattens it onto the form. A malformed
// or empty settings string falls back to the defaults so the form stays
// editable instead of blowing up.
function settingsToFormValues(initial: AwgOutbound): AwgOutboundFormValues {
  let parsed: Partial<AwgOutboundSettings> = {};
  try {
    if (initial.settings && initial.settings.trim()) {
      parsed = JSON.parse(initial.settings) as Partial<AwgOutboundSettings>;
    }
  } catch {
    parsed = {};
  }
  return {
    tag: initial.tag ?? '',
    remark: initial.remark ?? '',
    enable: initial.enable ?? true,
    privateKey: parsed.privateKey ?? DEFAULT_SETTINGS.privateKey,
    address: parsed.address ?? DEFAULT_SETTINGS.address,
    mtu: parsed.mtu ?? DEFAULT_SETTINGS.mtu,
    publicKey: parsed.publicKey ?? DEFAULT_SETTINGS.publicKey,
    psk: parsed.psk ?? DEFAULT_SETTINGS.psk,
    endpoint: parsed.endpoint ?? DEFAULT_SETTINGS.endpoint,
    keepalive: parsed.keepalive ?? DEFAULT_SETTINGS.keepalive,
    allowedIPs: parsed.allowedIPs ?? DEFAULT_SETTINGS.allowedIPs,
    dns: parsed.dns ?? DEFAULT_SETTINGS.dns,
    jc: parsed.jc ?? DEFAULT_SETTINGS.jc,
    jmin: parsed.jmin ?? DEFAULT_SETTINGS.jmin,
    jmax: parsed.jmax ?? DEFAULT_SETTINGS.jmax,
    s1: parsed.s1 ?? DEFAULT_SETTINGS.s1,
    s2: parsed.s2 ?? DEFAULT_SETTINGS.s2,
    s3: parsed.s3 ?? DEFAULT_SETTINGS.s3,
    s4: parsed.s4 ?? DEFAULT_SETTINGS.s4,
    h1: parsed.h1 ?? DEFAULT_SETTINGS.h1,
    h2: parsed.h2 ?? DEFAULT_SETTINGS.h2,
    h3: parsed.h3 ?? DEFAULT_SETTINGS.h3,
    h4: parsed.h4 ?? DEFAULT_SETTINGS.h4,
    i1: parsed.i1 ?? DEFAULT_SETTINGS.i1,
    i2: parsed.i2 ?? DEFAULT_SETTINGS.i2,
    i3: parsed.i3 ?? DEFAULT_SETTINGS.i3,
    i4: parsed.i4 ?? DEFAULT_SETTINGS.i4,
    i5: parsed.i5 ?? DEFAULT_SETTINGS.i5,
    headerProtectionKey: parsed.headerProtectionKey ?? DEFAULT_SETTINGS.headerProtectionKey,
    awgVersion: parsed.awgVersion ?? DEFAULT_SETTINGS.awgVersion,
    contentPaddingAddition: parsed.contentPaddingAddition ?? DEFAULT_SETTINGS.contentPaddingAddition,
    rekeyAfterTime: parsed.rekeyAfterTime ?? DEFAULT_SETTINGS.rekeyAfterTime,
    rekeyTimeout: parsed.rekeyTimeout ?? DEFAULT_SETTINGS.rekeyTimeout,
    rejectAfterTime: parsed.rejectAfterTime ?? DEFAULT_SETTINGS.rejectAfterTime,
    keepaliveTimeout: parsed.keepaliveTimeout ?? DEFAULT_SETTINGS.keepaliveTimeout,
    maxHandshakeAttempts: parsed.maxHandshakeAttempts ?? DEFAULT_SETTINGS.maxHandshakeAttempts,
    advancedSecurity: parsed.advancedSecurity ?? DEFAULT_SETTINGS.advancedSecurity,
  };
}

// formValuesToSettings packs the flattened form fields back into the settings
// object the backend expects. The settings field on the wire is a JSON STRING
// (per Task 8), so callers JSON.stringify the result before sending.
function formValuesToSettings(v: AwgOutboundFormValues): AwgOutboundSettings {
  return {
    privateKey: v.privateKey,
    address: v.address,
    mtu: v.mtu,
    publicKey: v.publicKey,
    psk: v.psk,
    endpoint: v.endpoint,
    keepalive: v.keepalive,
    allowedIPs: v.allowedIPs,
    dns: v.dns,
    jc: v.jc,
    jmin: v.jmin,
    jmax: v.jmax,
    s1: v.s1,
    s2: v.s2,
    s3: v.s3,
    s4: v.s4,
    h1: v.h1,
    h2: v.h2,
    h3: v.h3,
    h4: v.h4,
    i1: v.i1,
    i2: v.i2,
    i3: v.i3,
    i4: v.i4,
    i5: v.i5,
    headerProtectionKey: v.headerProtectionKey,
    awgVersion: v.awgVersion,
    contentPaddingAddition: v.contentPaddingAddition,
    rekeyAfterTime: v.rekeyAfterTime,
    rekeyTimeout: v.rekeyTimeout,
    rejectAfterTime: v.rejectAfterTime,
    keepaliveTimeout: v.keepaliveTimeout,
    maxHandshakeAttempts: v.maxHandshakeAttempts,
    advancedSecurity: v.advancedSecurity,
  };
}

export function AwgOutboundFormModal({ open, onClose, onSaved, initial }: Props) {
  const { t } = useTranslation();
  const [submitting, setSubmitting] = useState(false);
  const [pasteOpen, setPasteOpen] = useState(false);
  const [pasteText, setPasteText] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [messageApi, messageContextHolder] = message.useMessage();
  const isEdit = !!initial;

  const methods = useForm<AwgOutboundFormValues>({ defaultValues: buildDefaultValues() });

  const defaultValues = useMemo(() => {
    return initial ? settingsToFormValues(initial) : buildDefaultValues();
  }, [initial]);

  // Re-seed the form whenever the modal opens or the initial row changes.
  // methods.reset replaces the whole rhf store — the same pattern the Xray
  // outbound modal uses on open.
  useEffect(() => {
    if (!open) return;
    methods.reset(defaultValues);
    setShowAdvanced(false);
    setPasteText('');
  }, [open, defaultValues, methods]);

  const regenerateKeys = () => {
    const kp = Wireguard.generateKeypair();
    methods.setValue('privateKey', kp.privateKey, { shouldDirty: true });
    // The publicKey field on this form is the UPSTREAM server's public key,
    // not the local interface's — so we don't auto-derive it from the new
    // private key (that would overwrite the operator-entered upstream key).
  };

  async function onOk() {
    const valid = await methods.trigger();
    if (!valid) return;
    const values = methods.getValues();
    setSubmitting(true);
    try {
      const settings = formValuesToSettings(values);
      const payload: Partial<AwgOutbound> = {
        tag: values.tag.trim(),
        remark: values.remark ?? '',
        enable: values.enable ?? true,
        settings: JSON.stringify(settings),
      };
      if (isEdit && initial) {
        const full: AwgOutbound = {
          ...initial,
          ...payload,
          settings: payload.settings ?? '',
        } as AwgOutbound;
        await awgOutboundsApi.update(full);
      } else {
        await awgOutboundsApi.add(payload);
      }
      messageApi.success(t('pages.xray.awgOutbound.saved'));
      onSaved();
      onClose();
    } catch (e) {
      messageApi.error((e as Error)?.message || 'failed');
    } finally {
      setSubmitting(false);
    }
  }

  // handlePaste calls /parseConf to read an awg-quick .conf blob and inject
  // the parsed fields directly into the rhf store via methods.reset — the
  // brief's "TODO: inject res.obj" is implemented here so the feature works.
  async function handlePaste() {
    const text = pasteText.trim();
    if (!text) {
      messageApi.error(t('pages.xray.awgOutbound.pasteConf'));
      return;
    }
    try {
      const res = await awgOutboundsApi.parseConf(text);
      if (!res.success || !res.obj) {
        messageApi.error(res.msg || 'parse failed');
        return;
      }
      const parsed = res.obj as AwgOutboundSettings;
      const current = methods.getValues();
      const merged: AwgOutboundFormValues = {
        ...current,
        ...parsed,
      };
      methods.reset(merged);
      // Surface the obfuscation block if the parsed conf carried it so the
      // operator sees the values the parse produced — including HPK for a v3
      // .conf (which lives in the advanced section).
      const hasObf = parsed.jc || parsed.jmin || parsed.jmax || parsed.s1 || parsed.h1 || parsed.headerProtectionKey;
      if (hasObf) setShowAdvanced(true);
      setPasteOpen(false);
      messageApi.success(t('pages.xray.awgOutbound.parsed'));
    } catch (e) {
      messageApi.error((e as Error)?.message || 'parse failed');
    }
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={isEdit ? t('pages.xray.awgOutbound.edit') : t('pages.xray.awgOutbound.add')}
        onCancel={onClose}
        confirmLoading={submitting}
        onOk={onOk}
        okText={isEdit ? t('pages.clients.submitEdit') : t('create')}
        cancelText={t('close')}
        width={720}
        destroyOnHidden
      >
        <FormProvider {...methods}>
          <Form layout="vertical" colon={false}>
            <Row gutter={16}>
              <Col span={12}>
                <FormField
                  label={t('pages.xray.awgOutbound.tag')}
                  name="tag"
                  required
                  rules={{ required: t('pages.xray.outboundForm.tagRequired') }}
                >
                  <Input placeholder="awgo-1" />
                </FormField>
              </Col>
              <Col span={12}>
                <FormField label={t('pages.xray.awgOutbound.remark')} name="remark">
                  <Input />
                </FormField>
              </Col>
            </Row>

            <FormField label={t('pages.xray.awgOutbound.enable')} name="enable" valueProp="checked">
              <Switch />
            </FormField>

            <Row gutter={16}>
              <Col span={12}>
                <FormField
                  label={t('pages.xray.awgOutbound.endpoint')}
                  name="endpoint"
                  required
                  rules={{ required: t('pages.xray.outboundForm.addressRequired') }}
                >
                  <Input placeholder="up.example.com:51820" />
                </FormField>
              </Col>
              <Col span={12}>
                <FormField
                  label={t('pages.xray.awgOutbound.address')}
                  name="address"
                  required
                  extra={t('pages.xray.awgOutbound.addressHint')}
                  rules={{ required: t('pages.xray.outboundForm.addressRequired') }}
                >
                  <Input placeholder="10.9.0.5/32" />
                </FormField>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <FormField label={t('pages.xray.awgOutbound.privateKey')} name="privateKey">
                  <Input placeholder="(auto-generated)" />
                </FormField>
                <Button size="small" onClick={regenerateKeys}>
                  {t('pages.inbounds.form.awgRegenerate')}
                </Button>
              </Col>
              <Col span={12}>
                <FormField
                  label={t('pages.xray.awgOutbound.publicKey')}
                  name="publicKey"
                  required
                  rules={{ required: t('pages.xray.outboundForm.addressRequired') }}
                >
                  <Input placeholder="(upstream server public key)" />
                </FormField>
              </Col>
            </Row>

            <Row gutter={16}>
              <Col span={12}>
                <FormField label={t('pages.xray.awgOutbound.psk')} name="psk">
                  <Input />
                </FormField>
              </Col>
              <Col span={6}>
                <FormField label={t('pages.xray.awgOutbound.keepalive')} name="keepalive">
                  <InputNumber min={0} style={{ width: '100%' }} />
                </FormField>
              </Col>
              <Col span={6}>
                <FormField label={t('pages.xray.awgOutbound.mtu')} name="mtu">
                  <InputNumber min={576} max={65535} style={{ width: '100%' }} />
                </FormField>
              </Col>
            </Row>

            <FormField label={t('pages.xray.awgOutbound.allowedIPs')} name="allowedIPs">
              <Input placeholder="0.0.0.0/0, ::/0" />
            </FormField>

            {/* LUCX-HOOK: AWG outbound — protocol version. Determines which fields
                renderClientConf writes to the awgo-N .conf. Auto-detected by
                ParseConf when pasting; editable here. Version '3' gates HPK. */}
            <FormField
              label={t('pages.inbounds.form.awgVersion')}
              name="awgVersion"
              tooltip={t('pages.inbounds.form.awgVersionHint')}
            >
              <Select
                options={[
                  { value: '1.5', label: t('pages.inbounds.form.awgVersion15') },
                  { value: '2', label: t('pages.inbounds.form.awgVersion2') },
                  { value: '3', label: t('pages.inbounds.form.awgVersion3') },
                ]}
              />
            </FormField>
            {/* END LUCX-HOOK */}

            <Space>
              <Button onClick={() => setPasteOpen(true)}>{t('pages.xray.awgOutbound.pasteConf')}</Button>
              <Button type="link" onClick={() => setShowAdvanced((v) => !v)}>
                {t('pages.xray.awgOutbound.advanced')}
              </Button>
            </Space>

            {showAdvanced && (
              <>
                <FormField
                  label={t('pages.xray.awgOutbound.dns')}
                  name="dns"
                  extra={t('pages.xray.awgOutbound.dnsHint')}
                >
                  <Input placeholder="(optional — Xray resolves via UseIP by default)" />
                </FormField>
                <Row gutter={16}>
                  <Col span={6}>
                    <FormField name="jc" label="Jc">
                      <InputNumber style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col span={6}>
                    <FormField name="jmin" label="Jmin">
                      <InputNumber style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col span={6}>
                    <FormField name="jmax" label="Jmax">
                      <InputNumber style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col span={6}>
                    <FormField name="s1" label="S1">
                      <InputNumber style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                </Row>
                <Row gutter={16}>
                  <Col span={6}>
                    <FormField name="s2" label="S2">
                      <InputNumber style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col span={6}>
                    <FormField name="s3" label="S3">
                      <InputNumber style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                  <Col span={6}>
                    <FormField name="s4" label="S4">
                      <InputNumber style={{ width: '100%' }} />
                    </FormField>
                  </Col>
                </Row>
                <Row gutter={16}>
                  <Col span={6}>
                    <FormField name="h1" label="H1">
                      <Input />
                    </FormField>
                  </Col>
                  <Col span={6}>
                    <FormField name="h2" label="H2">
                      <Input />
                    </FormField>
                  </Col>
                  <Col span={6}>
                    <FormField name="h3" label="H3">
                      <Input />
                    </FormField>
                  </Col>
                  <Col span={6}>
                    <FormField name="h4" label="H4">
                      <Input />
                    </FormField>
                  </Col>
                </Row>
                <Row gutter={16}>
                  <Col span={5}>
                    <FormField name="i1" label="I1">
                      <Input />
                    </FormField>
                  </Col>
                  <Col span={5}>
                    <FormField name="i2" label="I2">
                      <Input />
                    </FormField>
                  </Col>
                  <Col span={5}>
                    <FormField name="i3" label="I3">
                      <Input />
                    </FormField>
                  </Col>
                  <Col span={5}>
                    <FormField name="i4" label="I4">
                      <Input />
                    </FormField>
                  </Col>
                  <Col span={4}>
                    <FormField name="i5" label="I5">
                      <Input />
                    </FormField>
                  </Col>
                </Row>
                {/* LUCX-HOOK: AWG outbound — HeaderProtectionKey (AWG3 only).
                    Shown in advanced regardless of the version selector so an
                    operator pasting a v3 conf sees the parsed value; it is only
                    written to the .conf when awgVersion === '3'. */}
                <FormField
                  label={t('pages.inbounds.form.awgHpk')}
                  name="headerProtectionKey"
                  tooltip={t('pages.inbounds.form.awgHpkHint')}
                >
                  <Input placeholder={t('pages.inbounds.form.awgHpkPlaceholder')} />
                </FormField>
                <Alert
                  type="warning"
                  showIcon
                  message={t('pages.inbounds.form.awgSRangeWarning')}
                  style={{ marginBottom: 16 }}
                />
                {/* AWG3 advanced timers/padding — 0 = kernel default. */}
                <Form.Item label={t('pages.inbounds.form.awgAdvancedSection')}>
                  <Space direction="vertical" style={{ width: '100%' }}>
                    <FormField name="contentPaddingAddition" label={t('pages.inbounds.form.awgContentPaddingAddition')} tooltip={t('pages.inbounds.form.awgContentPaddingAdditionHint')}>
                      <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0" />
                    </FormField>
                    <FormField name="rekeyAfterTime" label={t('pages.inbounds.form.awgRekeyAfterTime')} tooltip={t('pages.inbounds.form.awgRekeyAfterTimeHint')}>
                      <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0" />
                    </FormField>
                    <FormField name="rekeyTimeout" label={t('pages.inbounds.form.awgRekeyTimeout')} tooltip={t('pages.inbounds.form.awgRekeyTimeoutHint')}>
                      <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0" />
                    </FormField>
                    <FormField name="rejectAfterTime" label={t('pages.inbounds.form.awgRejectAfterTime')} tooltip={t('pages.inbounds.form.awgRejectAfterTimeHint')}>
                      <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0" />
                    </FormField>
                    <FormField name="keepaliveTimeout" label={t('pages.inbounds.form.awgKeepaliveTimeout')} tooltip={t('pages.inbounds.form.awgKeepaliveTimeoutHint')}>
                      <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0" />
                    </FormField>
                    <FormField name="maxHandshakeAttempts" label={t('pages.inbounds.form.awgMaxHandshakeAttempts')} tooltip={t('pages.inbounds.form.awgMaxHandshakeAttemptsHint')}>
                      <InputNumber min={0} max={65535} style={{ width: '100%' }} placeholder="0" />
                    </FormField>
                  </Space>
                </Form.Item>
                {/* AdvancedSecurity (AWG3 peer-level, advisory) on the upstream [Peer]. */}
                <FormField name="advancedSecurity" label={t('pages.inbounds.form.awgAdvancedSecurity')} tooltip={t('pages.inbounds.form.awgAdvancedSecurityHint')} valueProp="checked">
                  <Switch />
                </FormField>
                {/* END LUCX-HOOK */}
              </>
            )}
          </Form>
        </FormProvider>
      </Modal>

      <Drawer
        open={pasteOpen}
        onClose={() => setPasteOpen(false)}
        title={t('pages.xray.awgOutbound.pasteConfTitle')}
        extra={
          <Button type="primary" onClick={handlePaste}>
            {t('pages.xray.awgOutbound.parseAndFill')}
          </Button>
        }
      >
        <Input.TextArea
          rows={20}
          value={pasteText}
          onChange={(e) => setPasteText(e.target.value)}
          placeholder={'[Interface]\n...\n[Peer]\n...'}
        />
      </Drawer>
    </>
  );
}