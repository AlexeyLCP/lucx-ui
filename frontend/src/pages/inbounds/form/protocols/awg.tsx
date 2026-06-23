import { useTranslation } from 'react-i18next';
import { Button, Form, Input, InputNumber, Select, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { Wireguard } from '@/utils';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

// generateAwgObfuscation fills the Jc/Jmin/Jmax, S1-S4, H1-H4 fields for a
// given obfLevel, mirroring the Go GenerateAWGParams so the form can preview
// values before the backend persists them. The backend regenerates them on
// save when obfLevel/profile change, so this is for immediate display only.
function generateAwgObfuscation(level: number) {
  const r = (min: number, max: number) => min + Math.floor(Math.random() * (max - min + 1));
  const hexRange = (min: number, max: number) => `${min + Math.floor(Math.random() * (max - min))}`;
  let jc = 0, jmin = 0, jmax = 0;
  let s1 = 0, s2 = 0, s3 = 0, s4 = 0;
  let h1 = '', h2 = '', h3 = '', h4 = '';
  if (level >= 2) {
    jc = r(3, 10);
    jmin = r(50, 100);
    jmax = r(150, 250);
    s1 = r(20, 100); s2 = r(20, 100); s3 = r(20, 100); s4 = r(20, 100);
    h1 = hexRange(100000, 500000);
    h2 = hexRange(600000, 900000);
    h3 = hexRange(1000000, 1500000);
    h4 = hexRange(1600000, 2000000);
  }
  return { jc, jmin, jmax, s1, s2, s3, s4, h1, h2, h3, h4 };
}

export default function AwgFields() {
  const { t } = useTranslation();
  const form = Form.useFormInstance();
  const obfLevel = Form.useWatch(['settings', 'obfLevel'], form) as number | undefined;
  const routeThroughXray = Form.useWatch(['settings', 'routeThroughXray'], form) as boolean | undefined;
  const { data: outboundTags } = useOutboundTags();

  const regenerateKeys = () => {
    const kp = Wireguard.generateKeypair();
    const psk = Wireguard.generatePresharedKey();
    form.setFieldValue(['settings', 'privateKey'], kp.privateKey);
    form.setFieldValue(['settings', 'publicKey'], kp.publicKey);
    form.setFieldValue(['settings', 'presharedKey'], psk);
  };

  const regenerateObfuscation = () => {
    const level = (form.getFieldValue(['settings', 'obfLevel']) as number) ?? 2;
    const obf = generateAwgObfuscation(level);
    form.setFieldsValue({ settings: { ...obf } });
  };

  return (
    <>
      <Form.Item label={t('pages.inbounds.form.awgServerKeys')}>
        <Space.Compact block>
          <Form.Item name={['settings', 'privateKey']} noStyle>
            <Input readOnly placeholder={t('pages.inbounds.form.awgPrivateKey')} style={{ width: '50%' }} />
          </Form.Item>
          <Form.Item name={['settings', 'publicKey']} noStyle>
            <Input readOnly placeholder={t('pages.inbounds.form.awgPublicKey')} style={{ width: 'calc(50% - 32px)' }} />
          </Form.Item>
          <Button icon={<ReloadOutlined />} onClick={regenerateKeys} />
        </Space.Compact>
      </Form.Item>

      <Form.Item
        name={['settings', 'obfLevel']}
        label={t('pages.inbounds.form.awgObfLevel')}
        tooltip={t('pages.inbounds.form.awgObfLevelHint')}
      >
        <Select
          options={[
            { value: 1, label: '1 — none' },
            { value: 2, label: '2 — Jc/S/H' },
            { value: 3, label: '3 — full + CPS' },
          ]}
        />
      </Form.Item>

      <Form.Item name={['settings', 'mimicryProfile']} label={t('pages.inbounds.form.awgMimicryProfile')}>
        <Select
          options={[
            { value: 'quic', label: 'QUIC' },
            { value: 'sip', label: 'SIP' },
            { value: 'dns', label: 'DNS' },
          ]}
        />
      </Form.Item>

      <Form.Item name={['settings', 'mtu']} label={t('pages.inbounds.form.awgMtu')}>
        <InputNumber min={576} max={65535} style={{ width: '100%' }} />
      </Form.Item>

      <Form.Item name={['settings', 'dns']} label={t('pages.inbounds.form.awgDns')}>
        <Input placeholder="1.1.1.1, 1.0.0.1" />
      </Form.Item>

      <Form.Item label={t('pages.inbounds.form.awgObfuscation')}>
        <Button icon={<ReloadOutlined />} onClick={regenerateObfuscation}>
          {t('pages.inbounds.form.awgRegenerate')}
        </Button>
      </Form.Item>

      {(obfLevel ?? 2) >= 2 && (
        <>
          <Form.Item name={['settings', 'jc']} label="Jc">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 'jmin']} label="Jmin">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 'jmax']} label="Jmax">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 's1']} label="S1">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 's2']} label="S2">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 's3']} label="S3">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 's4']} label="S4">
            <InputNumber min={0} max={1000} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item name={['settings', 'h1']} label="H1">
            <Input placeholder="100000-500000" />
          </Form.Item>
          <Form.Item name={['settings', 'h2']} label="H2">
            <Input placeholder="600000-900000" />
          </Form.Item>
          <Form.Item name={['settings', 'h3']} label="H3">
            <Input placeholder="1000000-1500000" />
          </Form.Item>
          <Form.Item name={['settings', 'h4']} label="H4">
            <Input placeholder="1600000-2000000" />
          </Form.Item>
        </>
      )}

      {obfLevel === 3 && (
        <>
          <Form.Item name={['settings', 'i1']} label="I1">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
          <Form.Item name={['settings', 'i2']} label="I2">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
          <Form.Item name={['settings', 'i3']} label="I3">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
          <Form.Item name={['settings', 'i4']} label="I4">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
          <Form.Item name={['settings', 'i5']} label="I5">
            <Input placeholder={t('pages.inbounds.form.awgCpsHex')} />
          </Form.Item>
        </>
      )}

      <Form.Item
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.awgRouteThroughXray')}
        tooltip={t('pages.inbounds.form.awgRouteThroughXrayHint')}
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      {routeThroughXray && (
        <Form.Item
          name={['settings', 'outboundTag']}
          label={t('pages.inbounds.form.awgRouteOutbound')}
          tooltip={t('pages.inbounds.form.awgRouteOutboundHint')}
        >
          <Select
            allowClear
            showSearch
            placeholder={t('pages.inbounds.form.awgRouteOutboundPlaceholder')}
            options={(outboundTags ?? []).map((tag) => ({ value: tag, label: tag }))}
          />
        </Form.Item>
      )}
    </>
  );
}