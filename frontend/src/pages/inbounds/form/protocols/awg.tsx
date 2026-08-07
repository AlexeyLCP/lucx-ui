// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useMemo, useState } from 'react';
import { useFormContext } from 'react-hook-form';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Form, Input, InputNumber, message, Modal, Select, Space, Switch, Tag, Tooltip } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined, MedicineBoxOutlined, ReloadOutlined } from '@ant-design/icons';

import { FormField } from '@/components/form/rhf';
import { HttpUtil, Wireguard } from '@/utils';
import { useOutboundTags } from '@/api/queries/useOutboundTags';
import { maskSubnet, subnetsOverlap } from '@/lib/awg/subnet';
import { useAwgInboundId } from '../awg-inbound-id-context';

// LUCX-HOOK: AWG — map the panel obfLevel (1/2/3) and mimicryProfile to the
// backend cps package's profile enums. The backend owns the invariant-
// enforcing RNG (Jmin<Jmax, |S1+56-S2|>=10, H1-H4 in disjoint quadrants) and
// the CPS packet generators (TLS/DNS/SIP/QUIC), so the form calls the API
// instead of a local Math.random stub.
const OBF_PROFILE: Record<number, string> = { 1: 'lite', 2: 'standard', 3: 'pro' };

function levelToFullI1I5(level: number): boolean {
  return level >= 3;
}

// generateAwgObfuscationFromBackend calls /panel/api/inbounds/awg/generateObfuscation
// to get a fresh Jc/Jmin/Jmax/S1-S4/H1-H4 + I1-I5 set from the server. When the
// inbound targets AWG version '3', the response also carries a freshly generated
// HeaderProtectionKey (the generator guarantees S1-S4 >= 12).
async function generateAwgObfuscationFromBackend(getValue: (name: string) => unknown): Promise<Record<string, unknown> | null> {
  const level = (getValue('settings.obfLevel') as number) ?? 2;
  const mimicryProfile = (getValue('settings.mimicryProfile') as string) || 'quic';
  const browserProfile = (getValue('settings.browserProfile') as string) || 'chrome';
  const region = (getValue('settings.region') as string) || 'world';
  const awgVersion = (getValue('settings.awgVersion') as string) || '2';
  const obfProfile = OBF_PROFILE[level] ?? 'standard';
  const fullI1I5 = levelToFullI1I5(level);
  const msg = await HttpUtil.post('/panel/api/inbounds/awg/generateObfuscation', {
    obfProfile,
    mimicryProfile,
    browserProfile,
    region,
    domain: '',
    fullI1I5,
    awgVersion,
  }, { headers: { 'Content-Type': 'application/json' } });
  if (!msg?.success) return null;
  return (msg?.obj ?? null) as Record<string, unknown> | null;
}

// captureHostSignature captures a real QUIC handshake from the given domain.
async function captureHostSignature(domain: string): Promise<Record<string, string> | null> {
  const msg = await HttpUtil.post('/panel/api/inbounds/awg/captureHost', { domain }, { headers: { 'Content-Type': 'application/json' } });
  if (!msg?.success) return null;
  return (msg?.obj ?? null) as Record<string, string> | null;
}

// LUCX-HOOK: AWG — runtime diagnostics types, mirroring internal/awg/diagnostics.go.
interface AwgDiagCheck {
  name: string;
  ok: boolean;
  detail: string;
}

interface AwgDiagnostics {
  ifname: string;
  mode: string;
  healthy: boolean;
  checks: AwgDiagCheck[];
}

async function fetchAwgDiagnostics(id: number): Promise<AwgDiagnostics | null> {
  const msg = await HttpUtil.get(`/panel/api/inbounds/${id}/awgDiagnostics`);
  if (!msg?.success) return null;
  return (msg?.obj ?? null) as AwgDiagnostics | null;
}
// END LUCX-HOOK

export interface AwgFieldsProps {
  // otherAwgSubnets carries the masked network prefixes (e.g. "10.8.0.0/24")
  // of every OTHER enabled AWG inbound on this node, so the Address field can
  // warn when the operator types an overlapping subnet (kernel would install
  // two connected routes for the same prefix — see AGENTS.md Pattern 1e).
  otherAwgSubnets?: string[];
}

export default function AwgFields({ otherAwgSubnets = [] }: AwgFieldsProps) {
  const { t } = useTranslation();
  const [messageApi, messageContextHolder] = message.useMessage();
  // react-hook-form context (the inbound form is rhf, NOT AntD form). Use
  // useFormContext so setValue/getValue read/write the same store the
  // FormField/Controller bindings use — a plain AntD Form.useFormInstance()
  // would silently no-op against the rhf store (the root cause of the
  // "fields don't load/save" bug).
  const { setValue, watch } = useFormContext();
  const obfLevel = watch('settings.obfLevel') as number | undefined;
  const awgVersion = watch('settings.awgVersion') as '1.5' | '2' | '3' | undefined;
  const routeThroughXray = watch('settings.routeThroughXray') as boolean | undefined;
  const mimicryProfileVal = watch('settings.mimicryProfile') as string | undefined;
  const addressVal = watch('settings.address') as string | undefined;
  const { data: outboundTags } = useOutboundTags();

  // Detect a subnet collision: the operator's Address overlaps with another
  // AWG inbound's tunnel subnet. Advisory-only (yellow Alert, not a form
  // error) so existing dup-subnet inbounds still save — back-compat.
  const conflictSubnet = useMemo(() => {
    const self = maskSubnet(addressVal ?? '');
    if (!self) return null;
    return otherAwgSubnets.find((s) => subnetsOverlap(self, s)) ?? null;
  }, [addressVal, otherAwgSubnets]);
  const addressSubnetConflict = conflictSubnet !== null;

  const regenerateKeys = () => {
    const kp = Wireguard.generateKeypair();
    setValue('settings.privateKey', kp.privateKey, { shouldDirty: true });
    setValue('settings.publicKey', kp.publicKey, { shouldDirty: true });
  };

  // LUCX-HOOK: AWG — generate obfuscation via the backend (invariants + CPS).
  const [generating, setGenerating] = useState(false);
  const regenerateObfuscation = async () => {
    setGenerating(true);
    try {
      const obf = await generateAwgObfuscationFromBackend((name) => watch(name as never));
      if (!obf) {
        messageApi.error(t('pages.inbounds.form.awgRegenerateFailed'));
        return;
      }
      for (const [k, v] of Object.entries(obf)) {
        setValue(`settings.${k}`, v, { shouldDirty: true });
      }
      messageApi.success(t('pages.inbounds.form.awgRegenerateDone'));
    } finally {
      setGenerating(false);
    }
  };

  // LUCX-HOOK: AWG — capture a real QUIC handshake from a front domain.
  const [captureDomain, setCaptureDomain] = useState('');
  const [capturing, setCapturing] = useState(false);
  const captureHost = async () => {
    const dom = captureDomain.trim();
    if (!dom) {
      messageApi.error(t('pages.inbounds.form.awgCaptureDomainRequired'));
      return;
    }
    setCapturing(true);
    try {
      const res = await captureHostSignature(dom);
      if (!res || !res.i1) {
        messageApi.error(t('pages.inbounds.form.awgCaptureFailed'));
        return;
      }
      for (const k of ['i1', 'i2', 'i3', 'i4', 'i5']) {
        setValue(`settings.${k}`, (res as Record<string, string>)[k] ?? '', { shouldDirty: true });
      }
      messageApi.success(t('pages.inbounds.form.awgCaptureDone'));
    } finally {
      setCapturing(false);
    }
  };
  // END LUCX-HOOK

  // LUCX-HOOK: AWG — runtime diagnostics modal (interface/NAT/TUN probe chain).
  const inboundId = useAwgInboundId();
  const [diagOpen, setDiagOpen] = useState(false);
  const [diagLoading, setDiagLoading] = useState(false);
  const [diag, setDiag] = useState<AwgDiagnostics | null>(null);
  const loadDiagnostics = async () => {
    if (inboundId == null) return;
    setDiagLoading(true);
    try {
      const d = await fetchAwgDiagnostics(inboundId);
      if (!d) {
        messageApi.error(t('pages.inbounds.form.awgDiagFailed'));
        return;
      }
      setDiag(d);
      setDiagOpen(true);
    } finally {
      setDiagLoading(false);
    }
  };
  // END LUCX-HOOK

  return (
    <>
      {messageContextHolder}

      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.awgRouteThroughXray')}
        tooltip={t('pages.inbounds.form.awgRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <>
          <Alert
            type="info"
            showIcon
            className="mb-12"
            title={t('pages.inbounds.form.awgRouteThroughXrayNoReexport')}
          />
          <FormField
            name={['settings', 'outboundTag']}
            label={t('pages.inbounds.form.awgRouteOutbound')}
            tooltip={t('pages.inbounds.form.awgRouteOutboundHint')}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={[
                { value: '', label: t('pages.inbounds.form.awgRouteOutboundPlaceholder') },
                ...(outboundTags ?? []).map((tag) => ({ value: tag, label: tag })),
              ]}
            />
          </FormField>
        </>
      )}

      <Form.Item label={t('pages.inbounds.form.awgServerKeys')}>
        <Space.Compact block>
          <FormField name={['settings', 'privateKey']} noStyle>
            <Input readOnly placeholder={t('pages.inbounds.form.awgPrivateKey')} style={{ width: '50%' }} />
          </FormField>
          <FormField name={['settings', 'publicKey']} noStyle>
            <Input readOnly placeholder={t('pages.inbounds.form.awgPublicKey')} style={{ width: 'calc(50% - 32px)' }} />
          </FormField>
          <Button icon={<ReloadOutlined />} onClick={regenerateKeys} />
        </Space.Compact>
      </Form.Item>

      <FormField
        name={['settings', 'obfLevel']}
        label={t('pages.inbounds.form.awgObfLevel')}
        tooltip={t('pages.inbounds.form.awgObfLevelHint')}
      >
        <Select
          options={[
            { value: 1, label: 'Lite — лёгкая обфускация (Jc + DNS I1)' },
            { value: 2, label: 'Standard — средняя (Jc/S/H + TLS I1)' },
            { value: 3, label: 'Pro — полная (Jc/S/H + I1-I5)' },
          ]}
        />
      </FormField>

      {/* LUCX-HOOK: AWG — protocol version (the server-config ceiling). Sets
          which obfuscation fields the .conf carries: 1.5 (legacy), 2 (adds
          S3/S4 + I1-I5, Android 2.0.1), 3 (adds HeaderProtectionKey, desktop
          5.0.0.5 / Android 3.0.1). A v3 server will NOT accept v1/v2 clients. */}
      <FormField
        name={['settings', 'awgVersion']}
        label={t('pages.inbounds.form.awgVersion')}
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
      {awgVersion === '3' && (
        <Alert
          type="info"
          showIcon
          message={t('pages.inbounds.form.awgVersion3CompatNote')}
          style={{ marginBottom: 16 }}
        />
      )}
      {/* END LUCX-HOOK */}

      <FormField name={['settings', 'mimicryProfile']} label={t('pages.inbounds.form.awgMimicryProfile')} tooltip={t('pages.inbounds.form.awgMimicryProfileHint')}>
        <Select
          options={[
            { value: 'tls', label: 'TLS (ClientHello)' },
            { value: 'quic', label: 'QUIC (Initial packet)' },
            { value: 'dns', label: 'DNS (EDNS0 query)' },
            { value: 'sip', label: 'SIP (REGISTER)' },
          ]}
        />
      </FormField>

      {mimicryProfileVal === 'tls' && (
        <FormField name={['settings', 'browserProfile']} label={t('pages.inbounds.form.awgBrowserProfile')} tooltip={t('pages.inbounds.form.awgBrowserProfileHint')}>
          <Select
            options={[
              { value: 'chrome', label: t('pages.inbounds.form.awgBrowserChrome') },
              { value: 'firefox', label: t('pages.inbounds.form.awgBrowserFirefox') },
              { value: 'safari', label: t('pages.inbounds.form.awgBrowserSafari') },
            ]}
          />
        </FormField>
      )}

      <FormField name={['settings', 'region']} label={t('pages.inbounds.form.awgRegion')} tooltip={t('pages.inbounds.form.awgRegionHint')}>
        <Select
          options={[
            { value: 'ru', label: 'RU (включая РФ-сервисы: yandex/vk/gosuslugi)' },
            { value: 'world', label: 'World (только глобальные домены)' },
          ]}
        />
      </FormField>

      <FormField name={['settings', 'address']} label={t('pages.inbounds.form.awgAddress')} tooltip={t('pages.inbounds.form.awgAddressHint')}>
        <Input placeholder="10.200.0.1/24" />
      </FormField>

      {/* LUCX-HOOK: subnet-collision warning — two AWG inbounds on the same /24
          make the kernel install two connected routes for one prefix; reverse
          path to one inbound's clients egresses via the other (zombie) →
          "handshake ok, no traffic". Advisory (not a save blocker). */}
      {addressSubnetConflict && (
        <Alert
          type="warning"
          showIcon
          message={t('pages.inbounds.form.awgSubnetConflict')}
          description={t('pages.inbounds.form.awgSubnetConflictHint', { subnet: conflictSubnet ?? '' })}
          style={{ marginBottom: 16 }}
        />
      )}
      {/* END LUCX-HOOK */}

      <FormField name={['settings', 'mtu']} label={t('pages.inbounds.form.awgMtu')}>
        <InputNumber min={576} max={65535} style={{ width: '100%' }} />
      </FormField>

      <FormField name={['settings', 'dns']} label={t('pages.inbounds.form.awgDns')}>
        <Input placeholder="1.1.1.1, 1.0.0.1" />
      </FormField>

      <Form.Item label={t('pages.inbounds.form.awgObfuscation')}>
        <Button icon={<ReloadOutlined />} onClick={regenerateObfuscation} loading={generating}>
          {t('pages.inbounds.form.awgRegenerate')}
        </Button>
      </Form.Item>

      {/* LUCX-HOOK: AWG — host scan (QUIC capture → I1-I5, hoaxisr/awg-manager pattern) */}
      <Form.Item label={t('pages.inbounds.form.awgCaptureHost')} tooltip={t('pages.inbounds.form.awgCaptureHostHint')}>
        <Space.Compact style={{ display: 'flex' }}>
          <Input
            placeholder="google.com"
            value={captureDomain}
            onChange={(e) => setCaptureDomain(e.target.value)}
            style={{ flex: 1 }}
          />
          <Button onClick={captureHost} loading={capturing}>
            {t('pages.inbounds.form.awgCapture')}
          </Button>
        </Space.Compact>
      </Form.Item>
      {/* END LUCX-HOOK */}

      {/* LUCX-HOOK: AWG — runtime diagnostics (read-only probe of the live kernel state) */}
      <Form.Item label={t('pages.inbounds.form.awgDiagnostics')} tooltip={t('pages.inbounds.form.awgDiagnosticsHint')}>
        {inboundId == null ? (
          <Tooltip title={t('pages.inbounds.form.awgDiagSaveFirst')}>
            <Button icon={<MedicineBoxOutlined />} disabled>
              {t('pages.inbounds.form.awgDiagRun')}
            </Button>
          </Tooltip>
        ) : (
          <Button icon={<MedicineBoxOutlined />} onClick={loadDiagnostics} loading={diagLoading}>
            {t('pages.inbounds.form.awgDiagRun')}
          </Button>
        )}
      </Form.Item>
      <Modal
        open={diagOpen}
        title={
          <Space>
            {t('pages.inbounds.form.awgDiagTitle')}
            {diag && <Tag color="blue">{diag.ifname}</Tag>}
            {diag && <Tag>{diag.mode}</Tag>}
          </Space>
        }
        footer={
          <Button onClick={loadDiagnostics} loading={diagLoading}>
            {t('pages.inbounds.form.awgDiagRefresh')}
          </Button>
        }
        onCancel={() => setDiagOpen(false)}
      >
        {diag && (
          <>
            <Alert
              type={diag.healthy ? 'success' : 'warning'}
              showIcon
              message={diag.healthy ? t('pages.inbounds.form.awgDiagHealthy') : t('pages.inbounds.form.awgDiagUnhealthy')}
              style={{ marginBottom: 12 }}
            />
            {diag.checks.map((c) => (
              <div key={c.name} style={{ display: 'flex', gap: 8, marginBottom: 8, alignItems: 'flex-start' }}>
                {c.ok
                  ? <CheckCircleOutlined style={{ color: '#52c41a', marginTop: 4 }} />
                  : <CloseCircleOutlined style={{ color: '#ff4d4f', marginTop: 4 }} />}
                <div>
                  <div style={{ fontWeight: 600 }}>{c.name}</div>
                  <div style={{ opacity: 0.75, fontSize: 12, wordBreak: 'break-all' }}>{c.detail}</div>
                </div>
              </div>
            ))}
          </>
        )}
      </Modal>
      {/* END LUCX-HOOK */}

      {(obfLevel ?? 2) >= 2 && (
        <>
          <FormField name={['settings', 'jc']} label="Jc">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'jmin']} label="Jmin">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'jmax']} label="Jmax">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 's1']} label="S1">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 's2']} label="S2">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 's3']} label="S3">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 's4']} label="S4">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'h1']} label="H1">
            <Input placeholder="100000-500000" />
          </FormField>
          <FormField name={['settings', 'h2']} label="H2">
            <Input placeholder="600000-900000" />
          </FormField>
          <FormField name={['settings', 'h3']} label="H3">
            <Input placeholder="1000000-1500000" />
          </FormField>
          <FormField name={['settings', 'h4']} label="H4">
            <Input placeholder="1600000-2000000" />
          </FormField>
        </>
      )}

      {obfLevel === 3 && (
        <>
          <FormField name={['settings', 'i1']} label="I1">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
          <FormField name={['settings', 'i2']} label="I2">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
          <FormField name={['settings', 'i3']} label="I3">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
          <FormField name={['settings', 'i4']} label="I4">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
          <FormField name={['settings', 'i5']} label="I5">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </FormField>
        </>
      )}

      {/* LUCX-HOOK: AWG — HeaderProtectionKey is AWG3-only (version '3'). It is
          written to the .conf only when awgVersion === '3'; on v1/v2 the field
          stays empty so older kernels keep accepting the config. The generator
          guarantees S1-S4 >= 12 (required for the kernel to accept the key). */}
      {awgVersion === '3' && (
        <>
          <FormField
            name={['settings', 'headerProtectionKey']}
            label={t('pages.inbounds.form.awgHpk')}
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
          {/* AWG3 advanced timers/padding — all optional (0 = kernel default).
              Kernel defaults: RekeyAfterTime=120, RekeyTimeout=5,
              RejectAfterTime=180, KeepaliveTimeout=10, MaxHandshakeAttempts=18,
              ContentPaddingAddition=0. Operators override for DPI-tuning. */}
          <Form.Item label={t('pages.inbounds.form.awgAdvancedSection')}>
            <Space direction="vertical" style={{ width: '100%' }}>
              <FormField
                name={['settings', 'contentPaddingAddition']}
                label={t('pages.inbounds.form.awgContentPaddingAddition')}
                tooltip={t('pages.inbounds.form.awgContentPaddingAdditionHint')}
              >
                <Input style={{ width: '100%' }} placeholder="0 or 100-500" />
              </FormField>
              <FormField
                name={['settings', 'rekeyAfterTime']}
                label={t('pages.inbounds.form.awgRekeyAfterTime')}
                tooltip={t('pages.inbounds.form.awgRekeyAfterTimeHint')}
              >
                <Input style={{ width: '100%' }} placeholder="0 or 100-500" />
              </FormField>
              <FormField
                name={['settings', 'rekeyTimeout']}
                label={t('pages.inbounds.form.awgRekeyTimeout')}
                tooltip={t('pages.inbounds.form.awgRekeyTimeoutHint')}
              >
                <Input style={{ width: '100%' }} placeholder="0 or 100-500" />
              </FormField>
              <FormField
                name={['settings', 'rejectAfterTime']}
                label={t('pages.inbounds.form.awgRejectAfterTime')}
                tooltip={t('pages.inbounds.form.awgRejectAfterTimeHint')}
              >
                <Input style={{ width: '100%' }} placeholder="0 or 100-500" />
              </FormField>
              <FormField
                name={['settings', 'keepaliveTimeout']}
                label={t('pages.inbounds.form.awgKeepaliveTimeout')}
                tooltip={t('pages.inbounds.form.awgKeepaliveTimeoutHint')}
              >
                <Input style={{ width: '100%' }} placeholder="0 or 100-500" />
              </FormField>
              <FormField
                name={['settings', 'maxHandshakeAttempts']}
                label={t('pages.inbounds.form.awgMaxHandshakeAttempts')}
                tooltip={t('pages.inbounds.form.awgMaxHandshakeAttemptsHint')}
              >
                <Input style={{ width: '100%' }} placeholder="0 or 100-500" />
              </FormField>
            </Space>
          </Form.Item>
        </>
      )}
      {/* END LUCX-HOOK */}
    </>
  );
}
