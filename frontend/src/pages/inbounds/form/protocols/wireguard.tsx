import { useState } from 'react'; // LUCX-HOOK: kernel import preview
import { useFormContext } from 'react-hook-form'; // LUCX-HOOK: kernel backend fields
import { z } from 'zod'; // LUCX-HOOK: import response validation
import { useTranslation } from 'react-i18next';
import { Alert, Button, Form, Input, InputNumber, message, Modal, Space, Switch } from 'antd';
import { ReloadOutlined } from '@ant-design/icons';

import { FormField } from '@/components/form/rhf';
import { HttpUtil } from '@/utils'; // LUCX-HOOK: authenticated config preview
import { WireguardInboundSettingsSchema } from '@/schemas/protocols/inbound/wireguard'; // LUCX-HOOK
import WgoObfuscationFields from './wgo-obfuscation'; // LUCX-HOOK: wgobfs presets

interface WireguardFieldsProps {
  wgPubKey: string;
  regenInboundWg: () => void;
}

export default function WireguardFields({ wgPubKey, regenInboundWg }: WireguardFieldsProps) {
  const { t } = useTranslation();
  // LUCX-HOOK: explicit kernel selection never modifies an existing client profile.
  const form = useFormContext();
  const kernel = form.watch('settings.kernel') === true;
  const [importOpen, setImportOpen] = useState(false);
  const [configText, setConfigText] = useState('');
  const [busy, setBusy] = useState(false);
  const [messageApi, contextHolder] = message.useMessage();
  const previewImport = async () => {
    setBusy(true);
    try {
      const response = await HttpUtil.post(
        '/panel/api/inbounds/wireguard/importPreview',
        {
          config: configText,
          tproxyPort: Number(form.getValues('settings.tproxyPort')) || 32101,
        },
        { headers: { 'Content-Type': 'application/json' } },
      );
      if (!response?.success) return;
      const preview = z
        .object({
          port: z.number(),
          settings: z.union([z.string(), WireguardInboundSettingsSchema]),
          enable: z.literal(false),
        })
        .parse(response.obj);
      const settings = WireguardInboundSettingsSchema.parse(
        typeof preview.settings === 'string' ? JSON.parse(preview.settings) : preview.settings,
      );
      form.setValue('settings', settings, { shouldDirty: true, shouldValidate: true });
      form.setValue('port', preview.port, { shouldDirty: true });
      form.setValue('listen', '', { shouldDirty: true });
      form.setValue('enable', false, { shouldDirty: true });
      setImportOpen(false);
      setConfigText('');
    } catch {
      void messageApi.error(t('pages.xray.wireguard.kernelImportFailed'));
    } finally {
      setBusy(false);
    }
  };
  // END LUCX-HOOK
  return (
    <>
      {/* LUCX-HOOK: kernel backend fields and explicit safe import. */}
      {contextHolder}
      <FormField
        name={['settings', 'kernel']}
        label={t('pages.xray.wireguard.kernelBackend')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {kernel && (
        <>
          <Alert type="info" showIcon title={t('pages.xray.wireguard.kernelHint')} />
          <FormField name={['settings', 'address']} label={t('pages.xray.wireguard.kernelAddress')}>
            <Input placeholder="10.90.0.1/24" />
          </FormField>
          <FormField name={['settings', 'tproxyPort']} label={t('pages.xray.wireguard.kernelPort')}>
            <InputNumber min={1024} max={65535} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'fwMark']} label={t('pages.xray.wireguard.kernelFwMark')}>
            <Input placeholder="0xca6c" />
          </FormField>
          {/* LUCX-HOOK: wgobfs obfuscation presets (kernel WG only). */}
          <WgoObfuscationFields />
          <Button onClick={() => setImportOpen(true)}>
            {t('pages.xray.wireguard.kernelImport')}
          </Button>
          <Modal
            open={importOpen}
            title={t('pages.xray.wireguard.kernelImport')}
            onCancel={() => {
              setImportOpen(false);
              setConfigText('');
            }}
            onOk={() => void previewImport()}
            confirmLoading={busy}
          >
            <Alert type="warning" showIcon title={t('pages.xray.wireguard.kernelImportHint')} />
            <Input.TextArea
              value={configText}
              onChange={(event) => setConfigText(event.target.value)}
              rows={12}
              autoComplete="off"
              spellCheck={false}
            />
          </Modal>
        </>
      )}
      {/* END LUCX-HOOK */}
      <Form.Item label={t('pages.xray.wireguard.secretKey')}>
        <Space.Compact block>
          <FormField name={['settings', 'secretKey']} noStyle>
            <Input style={{ width: 'calc(100% - 32px)' }} />
          </FormField>
          <Button
            aria-label={t('regenerate')}
            icon={<ReloadOutlined aria-hidden />}
            onClick={regenInboundWg}
          />
        </Space.Compact>
      </Form.Item>
      <Form.Item label={t('pages.xray.wireguard.publicKey')}>
        <Input value={wgPubKey} disabled />
      </Form.Item>
      {!kernel && (
        <>
          <FormField name={['settings', 'subnetIp']} label={t('pages.xray.wireguard.subnetIp')}>
            <Input placeholder="10.0.0.0" />
          </FormField>
          <FormField name={['settings', 'subnetCidr']} label={t('pages.xray.wireguard.subnetCidr')}>
            <InputNumber min={1} max={32} style={{ width: '100%' }} />
          </FormField>
        </>
      )}
      <FormField name={['settings', 'mtu']} label="MTU">
        <InputNumber />
      </FormField>
      <FormField name={['settings', 'dns']} label={t('pages.inbounds.info.dns')}>
        <Input placeholder="1.1.1.1, 1.0.0.1" />
      </FormField>
      {!kernel && (
        <FormField
          name={['settings', 'noKernelTun']}
          label={t('pages.inbounds.info.noKernelTun')}
          valueProp="checked"
        >
          <Switch />
        </FormField>
      )}
    </>
  );
}
