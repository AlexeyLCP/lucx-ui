// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useState, type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, Button, Input, Radio, Upload, message } from 'antd';
import { UploadOutlined } from '@ant-design/icons';
import { useQuery } from '@tanstack/react-query';
import { useFormContext, useWatch } from 'react-hook-form';

import { FormField } from '@/components/form/rhf';
import { tunnelsApi } from '@/api/tunnels';
import { useAwgInboundId } from '../awg-inbound-id-context';

const SITE_PROMPT =
  'Create a distinctive, self-contained static website with three to five HTML pages, shared external CSS, an SVG favicon, a custom 404 page, and no remote resources. Use plain HTML/CSS and optional same-origin external JavaScript. Do not use inline script, inline CSS, forms, analytics, service workers, frames, or a client-side router. Output only deployable files, with index.html at the root and ordinary .html files for clean extensionless links. Do not mention Telegram or a proxy.';

export default function CoverFields() {
  const { t } = useTranslation();
  const inboundId = useAwgInboundId();
  const { setValue } = useFormContext();
  const siteSource = useWatch({ name: 'settings.siteSource' }) as string | undefined;
  const routes =
    (useWatch({ name: 'settings.routes' }) as { path?: string; dest?: string }[]) ?? [];
  const [busy, setBusy] = useState(false);
  const siteQuery = useQuery({
    queryKey: ['coverSiteFiles', inboundId],
    queryFn: async () => {
      const msg = await tunnelsApi.tproxySiteFiles(inboundId!);
      return msg.success && Array.isArray(msg.obj) ? msg.obj : [];
    },
    enabled: Boolean(inboundId) && siteSource === 'zip',
  });
  const siteFiles = siteQuery.data ?? [];

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
        await siteQuery.refetch();
        void message.success(t('pages.tunnels.tproxy.toasts.siteUploaded'));
      } finally {
        setBusy(false);
      }
    })();
    return false;
  };

  const routeText = routes
    .filter((r) => (r.path ?? '').trim() || (r.dest ?? '').trim())
    .map((r) => `${(r.path ?? '').trim()} ${(r.dest ?? '').trim()}`.trim())
    .join('\n');

  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 12 }}
        message={t('pages.inbounds.form.coverNote')}
      />
      <FormField
        name={['settings', 'hostname']}
        label={t('pages.inbounds.form.coverHostname')}
        tooltip={t('pages.inbounds.form.coverHostnameHint')}
        required
      >
        <Input placeholder="shop.example.com" />
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
          {siteFiles.length > 0 && (
            <div style={{ marginTop: 8, opacity: 0.75, fontSize: 12, whiteSpace: 'pre-wrap' }}>
              {siteFiles.join('\n')}
            </div>
          )}
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
      <FormItemLike>
        <div style={{ marginBottom: 8 }}>{t('pages.inbounds.form.coverRoutes')}</div>
        <Input.TextArea
          rows={3}
          placeholder=""
          value={routeText}
          onChange={(e) => {
            const parsed = e.target.value
              .split('\n')
              .map((line) => line.trim())
              .filter(Boolean)
              .map((line) => {
                const [path, dest] = line.split(/\s+/);
                return { path: path ?? '', dest: dest ?? '' };
              });
            setValue('settings.routes', parsed, { shouldDirty: true });
          }}
        />
        <div style={{ opacity: 0.65, fontSize: 12, marginTop: 4 }}>
          {t('pages.inbounds.form.coverRoutesHint')}
        </div>
      </FormItemLike>
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
