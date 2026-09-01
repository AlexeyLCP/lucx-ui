import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Input, InputNumber, Select, Switch, Alert } from 'antd';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { useOutboundTags } from '@/api/queries/useOutboundTags';

export default function OlcrtcFields() {
  const { t } = useTranslation();
  const { control, setValue } = useFormContext();
  const transport = useWatch({ control, name: 'settings.transport' }) as string | undefined;
  const provider = useWatch({ control, name: 'settings.provider' }) as string | undefined;
  const routeThroughXray = useWatch({ control, name: 'settings.routeThroughXray' }) as
    | boolean
    | undefined;
  const { data: outboundTags } = useOutboundTags();

  const transportOptions =
    provider === 'telemost'
      ? [
          { value: 'vp8channel', label: 'vp8channel' },
          { value: 'videochannel', label: 'videochannel' },
        ]
      : provider === 'wbstream'
        ? [
            { value: 'vp8channel', label: 'vp8channel' },
            { value: 'seichannel', label: 'seichannel' },
            { value: 'videochannel', label: 'videochannel' },
          ]
        : [
            { value: 'datachannel', label: 'datachannel' },
            { value: 'vp8channel', label: 'vp8channel' },
            { value: 'seichannel', label: 'seichannel' },
            { value: 'videochannel', label: 'videochannel' },
          ];

  useEffect(() => {
    const allowed =
      provider === 'telemost'
        ? ['vp8channel', 'videochannel']
        : provider === 'wbstream'
          ? ['vp8channel', 'seichannel', 'videochannel']
          : ['datachannel', 'vp8channel', 'seichannel', 'videochannel'];
    if (!transport || allowed.includes(transport)) return;
    setValue('settings.transport', 'vp8channel', { shouldDirty: true });
  }, [provider, transport, setValue]);

  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.olcrtcSingleCredNote')}
      />
      {provider === 'telemost' && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 12 }}
          message={t('pages.inbounds.form.olcrtcTelemostVp8Note')}
        />
      )}
      <FormField
        name={['settings', 'routeThroughXray']}
        label={t('pages.inbounds.form.olcrtcRouteThroughXray')}
        tooltip={t('pages.inbounds.form.olcrtcRouteThroughXrayHint')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
      {routeThroughXray && (
        <>
          <Alert
            type="warning"
            showIcon
            style={{ marginBottom: 12 }}
            message={t('pages.inbounds.form.olcrtcRouteThroughXrayWarn')}
          />
          <FormField
            name={['settings', 'outboundTag']}
            label={t('pages.inbounds.form.olcrtcRouteOutbound')}
            tooltip={t('pages.inbounds.form.olcrtcRouteOutboundHint')}
          >
            <Select
              showSearch
              optionFilterProp="label"
              options={[
                { value: '', label: t('pages.inbounds.form.olcrtcRouteOutboundPlaceholder') },
                ...(outboundTags || []).map((tag) => ({ value: tag, label: tag })),
              ]}
            />
          </FormField>
        </>
      )}
      <FormField name={['settings', 'provider']} label={t('pages.inbounds.form.olcrtcProvider')}>
        <Select
          options={[
            { value: 'jitsi', label: 'Jitsi' },
            { value: 'telemost', label: 'Yandex Telemost' },
            { value: 'wbstream', label: 'WB Stream' },
          ]}
        />
      </FormField>
      <FormField
        name={['settings', 'roomId']}
        label={t('pages.inbounds.form.olcrtcRoomId')}
        tooltip={t('pages.inbounds.form.olcrtcRoomIdHint')}
      >
        <Input placeholder="https://meet.jit.si/your-room" />
      </FormField>
      <FormField
        name={['settings', 'cryptoKey']}
        label={t('pages.inbounds.form.olcrtcCryptoKey')}
        tooltip={t('pages.inbounds.form.olcrtcCryptoKeyHint')}
      >
        <Input placeholder="openssl rand -hex 32" />
      </FormField>
      <FormField name={['settings', 'transport']} label={t('pages.inbounds.form.olcrtcTransport')}>
        <Select options={transportOptions} />
      </FormField>
      {transport === 'vp8channel' && (
        <>
          <FormField name={['settings', 'vp8Fps']} label={t('pages.inbounds.form.olcrtcVp8Fps')}>
            <InputNumber min={1} max={120} style={{ width: '100%' }} />
          </FormField>
          <FormField
            name={['settings', 'vp8Batch']}
            label={t('pages.inbounds.form.olcrtcVp8Batch')}
          >
            <InputNumber min={1} max={64} style={{ width: '100%' }} />
          </FormField>
        </>
      )}
      {transport === 'seichannel' && (
        <>
          <FormField name={['settings', 'seiFps']} label={t('pages.inbounds.form.olcrtcSeiFps')}>
            <InputNumber min={1} max={120} style={{ width: '100%' }} />
          </FormField>
          <FormField
            name={['settings', 'seiBatch']}
            label={t('pages.inbounds.form.olcrtcSeiBatch')}
          >
            <InputNumber min={1} max={64} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'seiFrag']} label={t('pages.inbounds.form.olcrtcSeiFrag')}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </FormField>
          <FormField name={['settings', 'seiAck']} label={t('pages.inbounds.form.olcrtcSeiAck')}>
            <InputNumber min={1} style={{ width: '100%' }} />
          </FormField>
        </>
      )}
      {transport === 'videochannel' && (
        <>
          <FormField
            name={['settings', 'videoW']}
            label={t('pages.inbounds.form.olcrtcVideoWidth')}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </FormField>
          <FormField
            name={['settings', 'videoH']}
            label={t('pages.inbounds.form.olcrtcVideoHeight')}
          >
            <InputNumber min={1} style={{ width: '100%' }} />
          </FormField>
          <FormField
            name={['settings', 'videoFps']}
            label={t('pages.inbounds.form.olcrtcVideoFps')}
          >
            <InputNumber min={1} max={120} style={{ width: '100%' }} />
          </FormField>
          <FormField
            name={['settings', 'videoCodec']}
            label={t('pages.inbounds.form.olcrtcVideoCodec')}
          >
            <Select
              options={[
                { value: 'qrcode', label: 'qrcode' },
                { value: 'tile', label: 'tile' },
              ]}
            />
          </FormField>
        </>
      )}
      <FormField name={['settings', 'dns']} label={t('pages.inbounds.form.olcrtcDns')}>
        <Input placeholder="8.8.8.8:53" />
      </FormField>
      <FormField
        name={['settings', 'debug']}
        label={t('pages.inbounds.form.olcrtcDebug')}
        valueProp="checked"
      >
        <Switch />
      </FormField>
    </>
  );
}
