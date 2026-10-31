import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, InputNumber, Select, message } from 'antd';

import { FormField } from '@/components/form/rhf';
import { MASQUE_DEFAULT_PATH } from '@/schemas/protocols/stream/masque';
import { useAwgInboundId } from '../awg-inbound-id-context';

export function MasqueSettingsFields() {
  const { t } = useTranslation();
  const inboundId = useAwgInboundId();
  const [messageApi, messageContextHolder] = message.useMessage();

  const exportDDM = async () => {
    if (!inboundId) {
      void messageApi.warning(t('pages.inbounds.form.masqueAppleRelaySaveFirst'));
      return;
    }
    const response = await fetch(`/panel/api/inbounds/${inboundId}/appleRelayDDM`, {
      method: 'POST',
      credentials: 'same-origin',
      headers: { 'Content-Type': 'application/json' },
      body: '{}',
    });
    if (!response.ok) {
      void messageApi.error(t('pages.inbounds.form.masqueAppleRelayExportFailed'));
      return;
    }
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = 'lucx-apple-network-relay.json';
    link.click();
    URL.revokeObjectURL(url);
  };

  return (
    <>
      {messageContextHolder}
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.masqueAppleRelayNote')}
        description={t('pages.inbounds.form.masqueAppleRelayNoteHelp')}
      />
      <FormField
        label={t('pages.inbounds.form.masque.addressPool')}
        tooltip={t('pages.inbounds.form.masque.addressPoolDesc')}
        name={['settings', 'address']}
        required
      >
        <Select mode="tags" tokenSeparators={[',', ' ']} open={false} />
      </FormField>
      <FormField label={t('pages.inbounds.info.mtu')} name={['settings', 'mtu']}>
        <InputNumber min={1280} max={65535} placeholder="1280" style={{ width: '100%' }} />
      </FormField>
      <Button onClick={() => void exportDDM()} disabled={!inboundId}>
        {t('pages.inbounds.form.masqueAppleRelayExport')}
      </Button>
    </>
  );
}

// The server matches requests on `path` alone; host, credentials and headers are
// the client's, delivered through the JSON subscription.
export default function MasqueStreamFields() {
  const { t } = useTranslation();
  return (
    <FormField label={t('path')} name={['streamSettings', 'masqueSettings', 'path']}>
      <Input placeholder={MASQUE_DEFAULT_PATH} />
    </FormField>
  );
}
