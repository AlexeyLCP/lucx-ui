import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Collapse, Modal, Select, Space, Spin, Tag, Typography } from 'antd';
import { HttpUtil } from '@/utils';
import { awgVersionAtLeast, awgVersionCeiling, isPostQuantumLink } from '@/lib/xray/inbound-link';
import type { AwgVersion } from '@/lib/xray/inbound-link';
import { LinkTags, linkMetaText, parseLinkParts } from '@/lib/xray/link-label';
import { QrPanel } from '@/pages/inbounds/qr';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { buildSubLinks, type SubSettingsLinks } from '@/lib/sub/links';
import {
  buildWireguardClientConfig,
  findWireguardInbound,
  isWireguardClient,
  buildAwgClientConfig,
  findAwgInbounds,
  isAwgClient,
} from './wireguardConfig';
import {
  buildAmneziaWGClientConfig,
  findAmneziaWGInbound,
  isAmneziaWGClient,
} from './amneziawgConfig';

type SubSettings = SubSettingsLinks;

interface ClientQrModalProps {
  open: boolean;
  client: ClientRecord | null;
  inboundsById: Record<number, InboundOption>;
  tunnelAllowedIPs?: Record<number, string>;
  subSettings?: SubSettings;
  onOpenChange: (open: boolean) => void;
}

interface ApiMsg<T = unknown> {
  success?: boolean;
  obj?: T;
}

const DEFAULT_SUB: SubSettings = {
  enable: false,
  subURI: '',
  subJsonURI: '',
  subJsonEnable: false,
  subClashURI: '',
  subClashEnable: false,
  subAwgURI: '',
  subAwgEnable: false,
  publicHost: '',
};

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
  tunnelAllowedIPs,
  subSettings = DEFAULT_SUB,
  onOpenChange,
}: ClientQrModalProps) {
  const { t } = useTranslation();
  const [links, setLinks] = useState<string[]>([]);
  const [loading, setLoading] = useState(false);

  const linksBuilt = useMemo(
    () => buildSubLinks(subSettings, client?.subId),
    [subSettings, client?.subId],
  );
  const subLink = linksBuilt.sub;
  const subJsonLink = linksBuilt.json;
  const subClashLink = linksBuilt.clash;
  const subAwgLink = linksBuilt.amnezia;
  const subAwgVpnLink = linksBuilt.amneziaVpn;

  const wgInbound = useMemo(
    () => findWireguardInbound(client, inboundsById),
    [client, inboundsById],
  );
  const wgConfigText = useMemo(() => {
    if (!client || !wgInbound || !isWireguardClient(client)) return '';
    return buildWireguardClientConfig(
      client,
      wgInbound,
      window.location.hostname,
      subSettings?.publicHost ?? '',
    );
  }, [client, wgInbound, subSettings?.publicHost]);

  // LUCX-HOOK: AWG — one .conf panel per attached AWG inbound. Each inbound has
  // its own awgVersion ceiling; a shared selector keyed off the first inbound
  // locked multi-attach clients to the lowest ceiling (AWG1=v1.5 disabled v2/v3).
  const awgInbounds = useMemo(() => findAwgInbounds(client, inboundsById), [client, inboundsById]);
  const [awgExportById, setAwgExportById] = useState<Record<number, AwgVersion>>({});
  useEffect(() => {
    setAwgExportById((prev) => {
      const next: Record<number, AwgVersion> = {};
      for (const ib of awgInbounds) {
        const ceiling = awgVersionCeiling(ib.awgVersion);
        next[ib.id] =
          prev[ib.id] && awgVersionAtLeast(ceiling, prev[ib.id]) ? prev[ib.id] : ceiling;
      }
      return next;
    });
  }, [awgInbounds]);
  const awgConfigs = useMemo(() => {
    if (!client || !isAwgClient(client))
      return [] as { ib: InboundOption; text: string; ceiling: AwgVersion; version: AwgVersion }[];
    const host = window.location.hostname;
    const pub = subSettings?.publicHost ?? '';
    return awgInbounds.map((ib) => {
      const ceiling = awgVersionCeiling(ib.awgVersion);
      const version = awgExportById[ib.id] ?? ceiling;
      return {
        ib,
        ceiling,
        version,
        text: buildAwgClientConfig(client, ib, host, pub, version),
      };
    });
  }, [client, awgInbounds, subSettings?.publicHost, awgExportById]);
  // END LUCX-HOOK

  const awgInbound = useMemo(
    () => findAmneziaWGInbound(client, inboundsById),
    [client, inboundsById],
  );
  const awgConfigText = useMemo(() => {
    if (!client || !awgInbound || !isAmneziaWGClient(client)) return '';
    const address = awgInbound ? (tunnelAllowedIPs?.[awgInbound.id] ?? '') : '';
    return buildAmneziaWGClientConfig(
      client,
      awgInbound,
      window.location.hostname,
      subSettings?.publicHost ?? '',
      address,
    );
  }, [client, awgInbound, tunnelAllowedIPs, subSettings?.publicHost]);

  const hasAnything =
    !!subLink ||
    !!subJsonLink ||
    !!subClashLink ||
    !!subAwgLink ||
    !!wgConfigText ||
    awgConfigs.length > 0 ||
    !!awgConfigText ||
    links.length > 0;

  // The reset runs during render so the effect only carries the request.
  const openSubId = open ? (client?.subId ?? '') : '';
  const [syncedSubId, setSyncedSubId] = useState(openSubId);
  if (openSubId !== syncedSubId) {
    setSyncedSubId(openSubId);
    setLinks([]);
    setLoading(!!openSubId);
  }

  useEffect(() => {
    if (!open || !client?.subId) return;
    let cancelled = false;
    (async () => {
      try {
        const msg = (await HttpUtil.get(
          `/panel/api/clients/subLinks/${encodeURIComponent(client.subId!)}`,
        )) as ApiMsg<string[]>;
        if (!cancelled) {
          setLinks(msg?.success && Array.isArray(msg.obj) ? msg.obj : []);
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [open, client?.subId]);

  const [activeKey, setActiveKey] = useState<string[]>([]);

  const items = useMemo(() => {
    const out: { key: string; label: React.ReactNode; children: React.ReactNode }[] = [];
    if (subLink) {
      out.push({
        key: 'sub',
        label: t('subscription.title'),
        children: (
          <QrPanel value={subLink} remark={`${client?.email || ''} — ${t('subscription.title')}`} />
        ),
      });
    }
    if (subJsonLink) {
      out.push({
        key: 'subJson',
        label: `${t('subscription.title')} (JSON)`,
        children: <QrPanel value={subJsonLink} remark={`${client?.email || ''} — JSON`} />,
      });
    }
    if (subClashLink) {
      out.push({
        key: 'subClash',
        label: (
          <Tag color="gold" style={{ margin: 0 }}>
            CLASH
          </Tag>
        ),
        children: <QrPanel value={subClashLink} remark={`${client?.email || ''} — Clash`} />,
      });
    }
    if (subAwgLink && awgConfigs.length > 0) {
      out.push({
        key: 'subAwg',
        label: (
          <Tag color="magenta" style={{ margin: 0 }}>
            AMNEZIA
          </Tag>
        ),
        children: <QrPanel value={subAwgLink} remark={`${client?.email || ''} — Amnezia .conf`} />,
      });
    }
    if (subAwgVpnLink && awgConfigs.length > 0) {
      out.push({
        key: 'subAwgVpn',
        label: (
          <Tag color="volcano" style={{ margin: 0 }}>
            vpn://
          </Tag>
        ),
        children: <QrPanel value={subAwgVpnLink} remark={`${client?.email || ''} — vpn://`} />,
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
      ) : (
        `${t('pages.clients.link')} ${idx + 1}`
      );
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
        label: (
          <Tag color="cyan" style={{ margin: 0 }}>
            {t('pages.clients.wireguardConfig')}
          </Tag>
        ),
        children: (
          <QrPanel
            value={wgConfigText}
            remark={client?.email || 'peer'}
            downloadName={`${client?.email || 'peer'}.conf`}
          />
        ),
      });
    }
    // LUCX-HOOK: AWG — one .conf panel per inbound (own ceiling + version selector).
    for (const cfg of awgConfigs) {
      const labelName = formatInboundLabel(cfg.ib.tag, cfg.ib.remark);
      out.push({
        key: `awg-config-${cfg.ib.id}`,
        label: (
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: 6, flexWrap: 'wrap' }}>
            <Tag color="purple" style={{ margin: 0 }}>
              {t('pages.clients.awgConfig')}
            </Tag>
            {labelName && <span style={{ opacity: 0.6, fontSize: 12 }}>({labelName})</span>}
          </span>
        ),
        children: (
          <Space direction="vertical" style={{ width: '100%' }} size="middle">
            <Space style={{ width: '100%', justifyContent: 'space-between' }} align="center">
              <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                {t('pages.clients.awgExportVersion')}
              </Typography.Text>
              <Select<AwgVersion>
                size="small"
                style={{ width: 180 }}
                value={cfg.version}
                onChange={(v) => setAwgExportById((prev) => ({ ...prev, [cfg.ib.id]: v }))}
                options={[
                  {
                    value: '1.5',
                    label: t('pages.inbounds.form.awgVersion15'),
                    disabled: !isVersionAvailable('1.5', cfg.ceiling),
                  },
                  {
                    value: '2',
                    label: t('pages.inbounds.form.awgVersion2'),
                    disabled: !isVersionAvailable('2', cfg.ceiling),
                  },
                  {
                    value: '3',
                    label: t('pages.inbounds.form.awgVersion3'),
                    disabled: !isVersionAvailable('3', cfg.ceiling),
                  },
                  {
                    value: '3.1',
                    label: t('pages.inbounds.form.awgVersion31'),
                    disabled: !isVersionAvailable('3.1', cfg.ceiling),
                  },
                ]}
              />
            </Space>
            <QrPanel
              value={cfg.text}
              remark={client?.email || 'peer'}
              downloadName={`${client?.email || 'peer'}-awg${cfg.ib.id}.conf`}
            />
          </Space>
        ),
      });
    }
    // END LUCX-HOOK
    if (awgConfigText) {
      out.push({
        key: 'amneziawg-config',
        label: (
          <Tag color="purple" style={{ margin: 0 }}>
            {t('pages.clients.amneziaWgConfig')}
          </Tag>
        ),
        children: (
          <QrPanel
            value={awgConfigText}
            remark={client?.email || 'peer'}
            downloadName={`${client?.email || 'peer'}.conf`}
          />
        ),
      });
    }
    return out;
  }, [
    subLink,
    subJsonLink,
    subClashLink,
    subAwgLink,
    subAwgVpnLink,
    wgConfigText,
    awgConfigs,
    awgConfigText,
    links,
    client?.email,
    t,
  ]);

  // Expanding the first panel is a render-time adjustment, not a side effect.
  const firstKey = open && items.length > 0 ? items[0].key : null;
  const [syncedFirstKey, setSyncedFirstKey] = useState<string | null>(null);
  if (firstKey !== syncedFirstKey) {
    setSyncedFirstKey(firstKey);
    setActiveKey(firstKey ? [firstKey] : []);
  }

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
          <div style={{ padding: 24, textAlign: 'center', opacity: 0.6 }}>
            {t('pages.clients.noSubId')}
          </div>
        )}
        {client?.subId && !hasAnything && !loading && (
          <div style={{ padding: 24, textAlign: 'center', opacity: 0.6 }}>
            {t('pages.clients.noLinks')}
          </div>
        )}
        {hasAnything && (
          <Collapse
            activeKey={activeKey}
            onChange={(keys) =>
              setActiveKey(typeof keys === 'string' ? [keys] : (keys as string[]))
            }
            items={items}
          />
        )}
      </Spin>
    </Modal>
  );
}
