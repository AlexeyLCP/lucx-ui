import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Collapse, Modal, Select, Space, Spin, Tag, Typography } from 'antd';
import { HttpUtil } from '@/utils';
import { awgVersionAtLeast, awgVersionCeiling, isPostQuantumLink } from '@/lib/xray/inbound-link';
import type { AwgVersion } from '@/lib/xray/inbound-link';
import { LinkTags, linkMetaText, parseLinkParts } from '@/lib/xray/link-label';
import { QrPanel } from '@/pages/inbounds/qr';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { buildWireguardClientConfig, findWireguardInbound, isWireguardClient, buildAwgClientConfig, findAwgInbound, isAwgClient } from './wireguardConfig'; // LUCX-HOOK: AWG

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
  publicHost?: string;
}

interface ClientQrModalProps {
  open: boolean;
  client: ClientRecord | null;
  inboundsById: Record<number, InboundOption>;
  subSettings?: SubSettings;
  onOpenChange: (open: boolean) => void;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

const DEFAULT_SUB: SubSettings = { enable: false, subURI: '', subJsonURI: '', subJsonEnable: false, publicHost: '' };

// isVersionAvailable reports whether an export version is selectable given the
// inbound ceiling (a client config may target any version at or below the
// server's). Mirrors the clamp logic in buildAwgClientConfig.
function isVersionAvailable(version: AwgVersion, ceiling: AwgVersion): boolean {
  return awgVersionAtLeast(ceiling, version);
}

export default function ClientQrModal({
  open,
  client,
  inboundsById,
  subSettings = DEFAULT_SUB,
  onOpenChange,
}: ClientQrModalProps) {
  const { t } = useTranslation();
  const [links, setLinks] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const subLink = useMemo(() => {
    if (!client?.subId || !subSettings?.enable || !subSettings?.subURI) return '';
    return subSettings.subURI + client.subId;
  }, [client?.subId, subSettings?.enable, subSettings?.subURI]);

  const subJsonLink = useMemo(() => {
    if (!client?.subId || !subSettings?.enable) return '';
    if (!subSettings?.subJsonEnable || !subSettings?.subJsonURI) return '';
    return subSettings.subJsonURI + client.subId;
  }, [client?.subId, subSettings?.enable, subSettings?.subJsonEnable, subSettings?.subJsonURI]);

  const wgInbound = useMemo(() => findWireguardInbound(client, inboundsById), [client, inboundsById]);
  const wgConfigText = useMemo(() => {
    if (!client || !wgInbound || !isWireguardClient(client)) return '';
    return buildWireguardClientConfig(client, wgInbound, window.location.hostname, subSettings?.publicHost ?? '');
  }, [client, wgInbound, subSettings?.publicHost]);

  // LUCX-HOOK: AWG — client .conf for AmneziaWG (with obfuscation block).
  const awgInbound = useMemo(() => findAwgInbound(client, inboundsById), [client, inboundsById]);
  // awgExportVersion is the runtime client-config version selector. Defaults to
  // the inbound ceiling; the dropdown offers every version at or below it so a
  // v3 inbound can still hand a v2/v1.5 client a config its app understands.
  const awgCeiling = useMemo(
    () => (awgInbound ? awgVersionCeiling(awgInbound.awgVersion) : '2'),
    [awgInbound],
  );
  const [awgExportVersion, setAwgExportVersion] = useState<AwgVersion>('2');
  useEffect(() => {
    setAwgExportVersion(awgCeiling);
  }, [awgCeiling]);
  const awgConfigText = useMemo(() => {
    if (!client || !awgInbound || !isAwgClient(client)) return '';
    return buildAwgClientConfig(client, awgInbound, window.location.hostname, subSettings?.publicHost ?? '', awgExportVersion);
  }, [client, awgInbound, subSettings?.publicHost, awgExportVersion]);
  // END LUCX-HOOK

  const hasAnything = !!subLink || !!subJsonLink || !!wgConfigText || !!awgConfigText || links.length > 0;

  useEffect(() => {
    if (!open || !client?.subId) {
      setLinks([]);
      return;
    }
    let cancelled = false;
    setLoading(true);
    (async () => {
      try {
        const msg = await HttpUtil.get(
          `/panel/api/clients/subLinks/${encodeURIComponent(client.subId!)}`,
        ) as ApiMsg<string[]>;
        if (!cancelled) {
          setLinks(msg?.success && Array.isArray(msg.obj) ? msg.obj : []);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => { cancelled = true; };
  }, [open, client?.subId]);

  const [activeKey, setActiveKey] = useState<string[]>([]);

  const items = useMemo(() => {
    const out: { key: string; label: React.ReactNode; children: React.ReactNode }[] = [];
    if (subLink) {
      out.push({
        key: 'sub',
        label: t('subscription.title'),
        children: <QrPanel value={subLink} remark={`${client?.email || ''} — ${t('subscription.title')}`} />,
      });
    }
    if (subJsonLink) {
      out.push({
        key: 'subJson',
        label: `${t('subscription.title')} (JSON)`,
        children: <QrPanel value={subJsonLink} remark={`${client?.email || ''} — JSON`} />,
      });
    }
    links.forEach((link, idx) => {
      const parts = parseLinkParts(link);
      const meta = parts ? linkMetaText(parts) : '';
      const label: React.ReactNode = parts ? (
        <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
          <LinkTags parts={parts} />
          {meta && <span style={{ opacity: 0.6, fontSize: 12 }}>({meta})</span>}
        </span>
      ) : `${t('pages.clients.link')} ${idx + 1}`;
      out.push({
        key: `l${idx}`,
        label,
        children: (
          <QrPanel
            value={link}
            remark={parts?.remark || `${client?.email || ''} #${idx + 1}`}
            showQr={!isPostQuantumLink(link)}
          />
        ),
      });
    });
    if (wgConfigText) {
      out.push({
        key: 'wg-config',
        label: <Tag color="cyan" style={{ margin: 0 }}>{t('pages.clients.wireguardConfig')}</Tag>,
        children: (
          <QrPanel
            value={wgConfigText}
            remark={client?.email || 'peer'}
            downloadName={`${client?.email || 'peer'}.conf`}
          />
        ),
      });
    }
    // LUCX-HOOK: AWG — client .conf panel with QR + download + export-version selector.
    if (awgConfigText) {
      out.push({
        key: 'awg-config',
        label: <Tag color="purple" style={{ margin: 0 }}>{t('pages.clients.awgConfig')}</Tag>,
        children: (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Space style={{ width: '100%', justifyContent: 'space-between' }} align="center">
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {t('pages.clients.awgExportVersion')}
              </Typography.Text>
              <Select<AwgVersion>
                size="small"
                style={{ width: 180 }}
                value={awgExportVersion}
                onChange={setAwgExportVersion}
                options={[
                  { value: '1.5', label: t('pages.inbounds.form.awgVersion15'), disabled: !isVersionAvailable('1.5', awgCeiling) },
                  { value: '2', label: t('pages.inbounds.form.awgVersion2'), disabled: !isVersionAvailable('2', awgCeiling) },
                  { value: '3', label: t('pages.inbounds.form.awgVersion3'), disabled: !isVersionAvailable('3', awgCeiling) },
                ]}
              />
            </Space>
            <QrPanel
              value={awgConfigText}
              remark={client?.email || 'peer'}
              downloadName={`${client?.email || 'peer'}-awg.conf`}
            />
          </Space>
        ),
      });
    }
    // END LUCX-HOOK
    return out;
  }, [subLink, subJsonLink, wgConfigText, awgConfigText, links, client?.email, t, awgExportVersion, awgCeiling]);

  useEffect(() => {
    if (!open) {
      setActiveKey([]);
      return;
    }
    setActiveKey(items.length > 0 ? [items[0].key] : []);
  }, [open, items]);

  return (
    <Modal
      open={open}
      title={client ? `${t('qrCode')} — ${client.email}` : t('qrCode')}
      footer={null}
      width={520}
      centered
      onCancel={() => onOpenChange(false)}
    >
      <Spin spinning={loading}>
        {!client?.subId && !loading && (
          <div style={{ padding: 24, textAlign: 'center', opacity: 0.6 }}>{t('pages.clients.noSubId')}</div>
        )}
        {client?.subId && !hasAnything && !loading && (
          <div style={{ padding: 24, textAlign: 'center', opacity: 0.6 }}>{t('pages.clients.noLinks')}</div>
        )}
        {hasAnything && (
          <Collapse
            activeKey={activeKey}
            onChange={(keys) => setActiveKey(typeof keys === 'string' ? [keys] : (keys as string[]))}
            items={items}
          />
        )}
      </Spin>
    </Modal>
  );
}
