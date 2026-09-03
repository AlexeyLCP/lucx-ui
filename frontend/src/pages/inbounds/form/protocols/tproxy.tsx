// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, Radio, Select, Upload, message } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import { useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { tunnelsApi } from '@/api/tunnels';
import { useAwgInboundId } from '../awg-inbound-id-context';

const SITE_PROMPT =
  'Create a distinctive, self-contained static website with three to five HTML pages, shared external CSS, an SVG favicon, a custom 404 page, and no remote resources. Use plain HTML/CSS and optional same-origin external JavaScript. Do not use inline script, inline CSS, forms, analytics, service workers, frames, or a client-side router. Output only deployable files, with index.html at the root and ordinary .html files for clean extensionless links. Do not mention Telegram or a proxy.';

export default function TproxyFields() {
  const { t } = useTranslation();
  const inboundId = useAwgInboundId();
  const siteSource = useWatch({ name: 'settings.siteSource' }) as string | undefined;
  const [busy, setBusy] = useState(false);

  const copyPrompt = async () => {
    await navigator.clipboard.writeText(SITE_PROMPT);
    void message.success(t('pages.inbounds.form.tproxySitePromptCopied'));
  };

  const uploadZip = (file: File) => {
    if (!inboundId) {
      void message.warning(t('pages.inbounds.form.tproxySaveFirst'));
      return false;
    }
    setBusy(true);
    void (async () => {
      try {
        await tunnelsApi.tproxyUploadSite(inboundId, file);
        void message.success(t('pages.tunnels.tproxy.toasts.siteUploaded'));
      } finally {
        setBusy(false);
      }
    })();
    return false;
  };

  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.tproxyNote')}
      />
      <FormField
        name={['settings', 'hostname']}
        label={t('pages.inbounds.form.tproxyHostname')}
        tooltip={t('pages.inbounds.form.tproxyHostnameHint')}
        required
      >
        <Input placeholder="proxy.example.com" />
      </FormField>
      <FormField
        name={['settings', 'secret']}
        label={t('pages.inbounds.form.tproxySecret')}
        tooltip={t('pages.inbounds.form.tproxySecretHint')}
      >
        <Input.Password autoComplete="new-password" />
      </FormField>
      <FormField
        name={['settings', 'siteSource']}
        label={t('pages.inbounds.form.tproxySiteSource')}
      >
        <Radio.Group>
          <Radio.Button value="zip">{t('pages.inbounds.form.tproxySiteZip')}</Radio.Button>
          <Radio.Button value="dir">{t('pages.inbounds.form.tproxySiteDir')}</Radio.Button>
          <Radio.Button value="upstream">
            {t('pages.inbounds.form.tproxySiteUpstream')}
          </Radio.Button>
        </Radio.Group>
      </FormField>
      {siteSource === 'zip' && (
        <FormItemLike>
          <Upload beforeUpload={uploadZip} maxCount={1} accept=".zip" disabled={busy}>
            <Button icon={<UploadOutlined />} loading={busy}>
              {t('pages.inbounds.form.tproxySiteZip')}
            </Button>
          </Upload>
          <Button type="link" onClick={() => void copyPrompt()}>
            {t('pages.inbounds.form.tproxySitePrompt')}
          </Button>
        </FormItemLike>
      )}
      {siteSource === 'dir' && (
        <FormField
          name={['settings', 'siteDir']}
          label={t('pages.inbounds.form.tproxySiteDir')}
          tooltip={t('pages.inbounds.form.tproxySiteDirHint')}
        >
          <Input placeholder="/var/www/mysite" />
        </FormField>
      )}
      {siteSource === 'upstream' && (
        <FormField
          name={['settings', 'siteUpstream']}
          label={t('pages.inbounds.form.tproxySiteUpstream')}
          tooltip={t('pages.inbounds.form.tproxySiteUpstreamHint')}
        >
          <Input placeholder="http://127.0.0.1:3000" />
        </FormField>
      )}
      <FormField name={['settings', 'carrierMode']} label={t('pages.inbounds.form.tproxyCarrier')}>
        <Select
          options={['https', 'https-lanes', 'websocket', 'websocket-lanes'].map((v) => ({
            value: v,
            label: v,
          }))}
        />
      </FormField>
      <FormField
        name={['settings', 'certFile']}
        label={t('pages.inbounds.form.trustTunnelCertFile')}
        tooltip={t('pages.inbounds.form.trustTunnelCertFileHint')}
      >
        <Input />
      </FormField>
      <FormField
        name={['settings', 'keyFile']}
        label={t('pages.inbounds.form.trustTunnelKeyFile')}
        tooltip={t('pages.inbounds.form.trustTunnelCertFileHint')}
      >
        <Input />
      </FormField>
    </>
  );
}

function FormItemLike({ children }: { children: ReactNode }) {
  return <div style={{ marginBottom: 16 }}>{children}</div>;
}
