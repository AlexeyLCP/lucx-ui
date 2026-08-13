import { useEffect, useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Divider, Modal, Popover, Select, Space, Tag, Tooltip, Typography, message } from 'antd';
import { CopyOutlined, DownloadOutlined, EyeOutlined, QrcodeOutlined, ReloadOutlined } from '@ant-design/icons';

import { ClipboardManager, FileManager, HttpUtil, IntlUtil, SizeFormatter } from '@/utils';
import { formatInboundLabel } from '@/lib/inbounds/label';
import { normalizeClientIps, type ClientIpInfo } from '@/lib/clients/ip-log';
import { useDatepicker } from '@/hooks/useDatepicker';
import type { ClientRecord, InboundOption } from '@/hooks/useClients';
import { awgVersionAtLeast, awgVersionCeiling, isPostQuantumLink } from '@/lib/xray/inbound-link';
import type { AwgVersion } from '@/lib/xray/inbound-link';
import { LinkTags, linkMetaText, parseLinkParts } from '@/lib/xray/link-label';
import { QrPanel } from '@/pages/inbounds/qr';
import ConfigBlock from '@/components/clients/ConfigBlock';
import { buildSubLinks } from '@/lib/sub/links';
import { buildWireguardClientConfig, findWireguardInbound, isWireguardClient, buildAwgClientConfig, findAwgInbounds, isAwgClient } from './wireguardConfig'; // LUCX-HOOK: AWG
import './ClientInfoModal.css';

const INBOUND_PROTOCOL_COLORS: Record<string, string> = {
  vless: 'blue',
  vmess: 'geekblue',
  trojan: 'volcano',
  shadowsocks: 'magenta',
  hysteria: 'cyan',
  hysteria2: 'green',
  wireguard: 'gold',
  http: 'purple',
  mixed: 'lime',
  tunnel: 'orange',
};

const INBOUND_CHIP_LIMIT = 1;

interface SubSettings {
  enable: boolean;
  subURI: string;
  subJsonURI: string;
  subJsonEnable: boolean;
  subClashURI: string;
  subClashEnable: boolean;
  subAwgURI?: string;
  subAwgEnable?: boolean;
  publicHost?: string;
}

interface ClientInfoModalProps {
  open: boolean;
  client: ClientRecord | null;
  inboundsById: Record<number, InboundOption>;
  isOnline: boolean;
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

const SUBSCRIPTION_DOWNLOAD_NAMES = {
  standard: 'subscription-standard.txt',
  json: 'subscription-json.json',
  clash: 'subscription-clash.yaml',
  amnezia: 'amneziawg.conf',
  amneziaVpn: 'amnezia-vpn.txt',
} as const;

export default function ClientInfoModal({
  open,
  client,
  inboundsById,
  isOnline,
  subSettings = DEFAULT_SUB,
  onOpenChange,
}: ClientInfoModalProps) {
  const { datepicker } = useDatepicker();
  const { t } = useTranslation();
  const expiryLabel = (ts?: number) => {
    if (!ts) return '∞';
    if (ts < 0) {
      const days = Math.round(ts / -86400000);
      return `${t('pages.clients.delayedStart')}: ${days}d`;
    }
    return IntlUtil.formatDate(ts, datepicker);
  };
  const dateLabel = (ts?: number) => (!ts || ts <= 0 ? '-' : IntlUtil.formatDate(ts, datepicker));
  const [messageApi, messageContextHolder] = message.useMessage();
  const [links, setLinks] = useState<string[]>([]);
  const [clientIps, setClientIps] = useState<ClientIpInfo[]>([]);
  const [ipsLoading, setIpsLoading] = useState(false);
  const [ipsClearing, setIpsClearing] = useState(false);
  const [ipsModalOpen, setIpsModalOpen] = useState(false);
  const [downloadingFormat, setDownloadingFormat] = useState<keyof typeof SUBSCRIPTION_DOWNLOAD_NAMES | null>(null);

  useEffect(() => {
    if (!open) {
      setLinks([]);
      setClientIps([]);
      setIpsModalOpen(false);
      return;
    }
    if (!client?.subId) return;
    let cancelled = false;
    (async () => {
      const msg = await HttpUtil.get(
        `/panel/api/clients/subLinks/${encodeURIComponent(client.subId!)}`,
      ) as ApiMsg<string[]>;
      if (cancelled) return;
      setLinks(msg?.success && Array.isArray(msg.obj) ? msg.obj : []);
    })();
    return () => { cancelled = true; };
  }, [open, client?.subId]);

  const traffic = client?.traffic || null;
  const totalBytes = client?.totalGB || 0;
  const used = (traffic?.up || 0) + (traffic?.down || 0);
  const remaining = useMemo(() => {
    if (totalBytes <= 0) return -1;
    const r = totalBytes - used;
    return r > 0 ? r : 0;
  }, [totalBytes, used]);

  const linksBuilt = useMemo(
    () => buildSubLinks(subSettings, client?.subId),
    [subSettings, client?.subId],
  );
  const subLink = linksBuilt.sub;
  const subJsonLink = linksBuilt.json;
  const subClashLink = linksBuilt.clash;
  const subAwgLink = linksBuilt.amnezia;
  const subAwgVpnLink = linksBuilt.amneziaVpn;

  const showSubscription = !!(client?.subId && (subLink || subJsonLink || subClashLink || subAwgLink));
  const wgInbound = useMemo(() => findWireguardInbound(client, inboundsById), [client, inboundsById]);
  const wgConfigText = useMemo(() => {
    if (!client || !wgInbound || !isWireguardClient(client)) return '';
    return buildWireguardClientConfig(client, wgInbound, window.location.hostname, subSettings?.publicHost ?? '');
  }, [client, wgInbound, subSettings?.publicHost]);
  // LUCX-HOOK: AWG — one .conf per attached inbound (own ceiling + version select).
  const awgInbounds = useMemo(() => findAwgInbounds(client, inboundsById), [client, inboundsById]);
  const [awgExportById, setAwgExportById] = useState<Record<number, AwgVersion>>({});
  useEffect(() => {
    setAwgExportById((prev) => {
      const next: Record<number, AwgVersion> = {};
      for (const ib of awgInbounds) {
        const ceiling = awgVersionCeiling(ib.awgVersion);
        next[ib.id] = prev[ib.id] && awgVersionAtLeast(ceiling, prev[ib.id]) ? prev[ib.id] : ceiling;
      }
      return next;
    });
  }, [awgInbounds]);
  const awgConfigs = useMemo(() => {
    if (!client || !isAwgClient(client)) return [] as { ib: InboundOption; text: string; ceiling: AwgVersion; version: AwgVersion }[];
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

  async function copyValue(text: string) {
    if (!text) return;
    try {
      const { fetchSubscriptionBody, isAmneziaVpnUrl } = await import('@/lib/sub/fetchBody');
      // Amnezia vpn:// row is stored as HTTPS ?format=vpn URL — copy the body.
      const payload = isAmneziaVpnUrl(text) ? await fetchSubscriptionBody(text) : text;
      const ok = await ClipboardManager.copyText(payload);
      if (ok) messageApi.success(t('copied'));
    } catch {
      messageApi.error(t('somethingWentWrong'));
    }
  }

  async function downloadSubscription(url: string, format: keyof typeof SUBSCRIPTION_DOWNLOAD_NAMES) {
    if (!url || downloadingFormat) return;
    setDownloadingFormat(format);
    try {
      const response = await fetch(url);
      if (!response.ok) throw new Error('Subscription download failed');
      const content = await response.text();
      FileManager.downloadTextFile(content, SUBSCRIPTION_DOWNLOAD_NAMES[format]);
    } catch (_) {
      messageApi.error(t('somethingWentWrong'));
    } finally {
      setDownloadingFormat(null);
    }
  }

  async function loadIps() {
    if (!client?.email) return;
    setIpsLoading(true);
    try {
      const msg = await HttpUtil.post(`/panel/api/clients/ips/${encodeURIComponent(client.email)}`) as ApiMsg<unknown[]>;
      if (!msg?.success) { setClientIps([]); return; }
      setClientIps(normalizeClientIps(msg.obj));
    } finally {
      setIpsLoading(false);
    }
  }

  async function clearIps() {
    if (!client?.email) return;
    setIpsClearing(true);
    try {
      const msg = await HttpUtil.post(`/panel/api/clients/clearIps/${encodeURIComponent(client.email)}`) as ApiMsg;
      if (msg?.success) setClientIps([]);
    } finally {
      setIpsClearing(false);
    }
  }

  function openIpsModal() {
    setIpsModalOpen(true);
    if (clientIps.length === 0) void loadIps();
  }

  return (
    <>
      {messageContextHolder}
      <Modal
        open={open}
        title={client ? `${t('pages.clients.clientInfo')} — ${client.email}` : t('pages.clients.clientInfo')}
        footer={null}
        width={640}
        onCancel={() => onOpenChange(false)}
      >
        {client && (
          <>
            <table className="info-table block">
              <tbody>
                <tr>
                  <td>{t('pages.clients.online')}</td>
                  <td>
                    {client.enable && isOnline
                      ? <Tag color="green">{t('pages.clients.online')}</Tag>
                      : <Tag>{t('pages.clients.offline')}</Tag>}
                    <span className="hint">{t('lastOnline')}: {dateLabel(traffic?.lastOnline)}</span>
                  </td>
                </tr>
                <tr>
                  <td>{t('status')}</td>
                  <td>
                    <Tag color={client.enable ? 'green' : 'default'}>
                      {client.enable ? t('enabled') : t('disabled')}
                    </Tag>
                  </td>
                </tr>
                <tr>
                  <td>{t('pages.clients.email')}</td>
                  <td>
                    {client.email
                      ? <Tag color="green">{client.email}</Tag>
                      : <Tag color="red">{t('none')}</Tag>}
                  </td>
                </tr>
                <tr>
                  <td>{t('pages.clients.subId')}</td>
                  <td>
                    <Tag className="info-large-tag">{client.subId || '-'}</Tag>
                    {client.subId && (
                      <Button size="small" type="text" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(client.subId!)} />
                    )}
                  </td>
                </tr>
                {client.uuid && (
                  <tr>
                    <td>{t('pages.clients.uuid')}</td>
                    <td>
                      <Tag className="info-large-tag">{client.uuid}</Tag>
                      <Button size="small" type="text" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(client.uuid!)} />
                    </td>
                  </tr>
                )}
                {client.password && (
                  <tr>
                    <td>{t('password')}</td>
                    <td>
                      <Tag className="info-large-tag">{client.password}</Tag>
                      <Button size="small" type="text" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(client.password!)} />
                    </td>
                  </tr>
                )}
                {client.auth && (
                  <tr>
                    <td>{t('pages.clients.auth')}</td>
                    <td>
                      <Tag className="info-large-tag">{client.auth}</Tag>
                      <Button size="small" type="text" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(client.auth!)} />
                    </td>
                  </tr>
                )}
                <tr>
                  <td>{t('pages.clients.flow')}</td>
                  <td>
                    {client.flow ? <Tag>{client.flow}</Tag> : <Tag color="orange">{t('none')}</Tag>}
                  </td>
                </tr>
                <tr>
                  <td>{t('pages.inbounds.traffic')}</td>
                  <td>
                    <Tag>
                      ↑ {SizeFormatter.sizeFormat(traffic?.up || 0)}
                      {' '}/ ↓ {SizeFormatter.sizeFormat(traffic?.down || 0)}
                    </Tag>
                    <span className="hint">
                      {SizeFormatter.sizeFormat(used)} / {totalBytes > 0 ? SizeFormatter.sizeFormat(totalBytes) : '∞'}
                    </span>
                  </td>
                </tr>
                <tr>
                  <td>{t('remained')}</td>
                  <td>
                    {remaining < 0
                      ? <Tag color="purple">∞</Tag>
                      : <Tag color={remaining > 0 ? '' : 'red'}>{SizeFormatter.sizeFormat(remaining)}</Tag>}
                  </td>
                </tr>
                <tr>
                  <td>{t('pages.inbounds.expireDate')}</td>
                  <td>
                    {!client.expiryTime
                      ? <Tag color="purple">∞</Tag>
                      : <Tag color={client.expiryTime < 0 ? 'blue' : undefined}>{expiryLabel(client.expiryTime)}</Tag>}
                    {(client.expiryTime ?? 0) > 0 && (
                      <span className="hint">{IntlUtil.formatRelativeTime(client.expiryTime)}</span>
                    )}
                  </td>
                </tr>
                <tr>
                  <td>{t('pages.clients.ipLimit')}</td>
                  <td>{!client.limitIp ? <Tag>∞</Tag> : <Tag>{client.limitIp}</Tag>}</td>
                </tr>
                <tr>
                  <td>{t('pages.inbounds.IPLimitlog')}</td>
                  <td>
                    <Button size="small" icon={<EyeOutlined />} aria-label={t('pages.clients.ipLog')} loading={ipsLoading} onClick={openIpsModal}>
                      {clientIps.length > 0 ? clientIps.length : ''}
                    </Button>
                  </td>
                </tr>
                <tr>
                  <td>{t('pages.inbounds.createdAt')}</td>
                  <td><Tag>{dateLabel(client.createdAt)}</Tag></td>
                </tr>
                <tr>
                  <td>{t('pages.inbounds.updatedAt')}</td>
                  <td><Tag>{dateLabel(client.updatedAt)}</Tag></td>
                </tr>
                {client.group && (
                  <tr>
                    <td>{t('pages.clients.group')}</td>
                    <td><Tag color="geekblue">{client.group}</Tag></td>
                  </tr>
                )}
                {client.comment && (
                  <tr>
                    <td>{t('pages.clients.comment')}</td>
                    <td><Tag className="info-large-tag">{client.comment}</Tag></td>
                  </tr>
                )}
                <tr>
                  <td>{t('pages.clients.attachedInbounds')}</td>
                  <td>
                    {(() => {
                      const ids = client.inboundIds || [];
                      if (ids.length === 0) return <span className="hint">—</span>;
                      const visible = ids.slice(0, INBOUND_CHIP_LIMIT);
                      const overflow = ids.slice(INBOUND_CHIP_LIMIT);
                      const inboundChip = (id: number) => {
                        const ib = inboundsById[id];
                        const proto = (ib?.protocol || '').toLowerCase();
                        const color = INBOUND_PROTOCOL_COLORS[proto] ?? 'default';
                        const label = formatInboundLabel(ib?.tag, ib?.remark);
                        return (
                          <Tooltip key={id} title={label}>
                            <Tag color={color}>{label}</Tag>
                          </Tooltip>
                        );
                      };
                      return (
                        <div className="chips">
                          {visible.map((id) => inboundChip(id))}
                          {overflow.length > 0 && (
                            <Popover
                              trigger="click"
                              placement="bottomRight"
                              content={
                                <div className="chips chips-stack">
                                  {overflow.map((id) => inboundChip(id))}
                                </div>
                              }
                            >
                              <Tag color="default" className="chip-more">
                                +{overflow.length} {t('more') !== 'more' ? t('more') : 'more'}
                              </Tag>
                            </Popover>
                          )}
                        </div>
                      );
                    })()}
                  </td>
                </tr>
              </tbody>
            </table>

            {showSubscription && client && (
              <>
                <Divider>{t('subscription.title')}</Divider>
                {subLink && (
                  <div className="link-row">
                    <Tag color="green" className="link-row-tag">SUB</Tag>
                    <a href={subLink} target="_blank" rel="noopener noreferrer" className="link-row-title link-row-title-anchor" title={subLink}>
                      {client.subId}
                    </a>
                    <div className="link-row-actions">
                      <Tooltip title={t('copy')}>
                        <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(subLink)} />
                      </Tooltip>
                      <Tooltip title={t('download')}>
                        <Button size="small" icon={<DownloadOutlined />} aria-label={t('download')} loading={downloadingFormat === 'standard'} disabled={downloadingFormat !== null} onClick={() => void downloadSubscription(subLink, 'standard')} />
                      </Tooltip>
                      <Popover trigger="click" placement="left" destroyOnHidden content={<QrPanel value={subLink} remark={`${client.email} — ${t('subscription.title')}`} size={220} />}>
                        <Tooltip title={t('pages.clients.qrCode')}>
                          <Button size="small" icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
                        </Tooltip>
                      </Popover>
                    </div>
                  </div>
                )}
                {subJsonLink && (
                  <div className="link-row">
                    <Tag color="purple" className="link-row-tag">JSON</Tag>
                    <a href={subJsonLink} target="_blank" rel="noopener noreferrer" className="link-row-title link-row-title-anchor" title={subJsonLink}>
                      {client.subId}
                    </a>
                    <div className="link-row-actions">
                      <Tooltip title={t('copy')}>
                        <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(subJsonLink)} />
                      </Tooltip>
                      <Tooltip title={t('download')}>
                        <Button size="small" icon={<DownloadOutlined />} aria-label={t('download')} loading={downloadingFormat === 'json'} disabled={downloadingFormat !== null} onClick={() => void downloadSubscription(subJsonLink, 'json')} />
                      </Tooltip>
                      <Popover trigger="click" placement="left" destroyOnHidden content={<QrPanel value={subJsonLink} remark={`${client.email} — JSON`} size={220} />}>
                        <Tooltip title={t('pages.clients.qrCode')}>
                          <Button size="small" icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
                        </Tooltip>
                      </Popover>
                    </div>
                  </div>
                )}
                {subClashLink && (
                  <div className="link-row">
                    <Tooltip title="Clash / Mihomo (+ AWG)">
                      <Tag color="gold" className="link-row-tag">CLASH</Tag>
                    </Tooltip>
                    <a href={subClashLink} target="_blank" rel="noopener noreferrer" className="link-row-title link-row-title-anchor" title={subClashLink}>
                      {client.subId}
                    </a>
                    <div className="link-row-actions">
                      <Tooltip title={t('copy')}>
                        <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(subClashLink)} />
                      </Tooltip>
                      <Tooltip title={t('download')}>
                        <Button size="small" icon={<DownloadOutlined />} aria-label={t('download')} loading={downloadingFormat === 'clash'} disabled={downloadingFormat !== null} onClick={() => void downloadSubscription(subClashLink, 'clash')} />
                      </Tooltip>
                      <Popover trigger="click" placement="left" destroyOnHidden content={<QrPanel value={subClashLink} remark={`${client.email} — Clash / Mihomo`} size={220} />}>
                        <Tooltip title={t('pages.clients.qrCode')}>
                          <Button size="small" icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
                        </Tooltip>
                      </Popover>
                    </div>
                  </div>
                )}
                {subAwgLink && (
                  <div className="link-row">
                    <Tooltip title={t('pages.clients.subAwgHint')}>
                      <Tag color="magenta" className="link-row-tag">AMNEZIA</Tag>
                    </Tooltip>
                    <a href={subAwgLink} target="_blank" rel="noopener noreferrer" className="link-row-title link-row-title-anchor" title={subAwgLink}>
                      {client.subId}
                    </a>
                    <div className="link-row-actions">
                      <Tooltip title={t('copy')}>
                        <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(subAwgLink)} />
                      </Tooltip>
                      <Tooltip title={t('download')}>
                        <Button size="small" icon={<DownloadOutlined />} aria-label={t('download')} loading={downloadingFormat === 'amnezia'} disabled={downloadingFormat !== null} onClick={() => void downloadSubscription(subAwgLink, 'amnezia')} />
                      </Tooltip>
                      <Popover trigger="click" placement="left" destroyOnHidden content={<QrPanel value={subAwgLink} remark={`${client.email} — Amnezia .conf`} size={220} />}>
                        <Tooltip title={t('pages.clients.qrCode')}>
                          <Button size="small" icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
                        </Tooltip>
                      </Popover>
                    </div>
                  </div>
                )}
                {subAwgVpnLink && (
                  <div className="link-row">
                    <Tooltip title={t('pages.clients.subAwgVpnHint')}>
                      <Tag color="volcano" className="link-row-tag">vpn://</Tag>
                    </Tooltip>
                    <a href={subAwgVpnLink} target="_blank" rel="noopener noreferrer" className="link-row-title link-row-title-anchor" title={subAwgVpnLink}>
                      {client.subId}?format=vpn
                    </a>
                    <div className="link-row-actions">
                      <Tooltip title={t('copy')}>
                        <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(subAwgVpnLink)} />
                      </Tooltip>
                      <Tooltip title={t('download')}>
                        <Button size="small" icon={<DownloadOutlined />} aria-label={t('download')} loading={downloadingFormat === 'amneziaVpn'} disabled={downloadingFormat !== null} onClick={() => void downloadSubscription(subAwgVpnLink, 'amneziaVpn')} />
                      </Tooltip>
                      <Popover trigger="click" placement="left" destroyOnHidden content={<QrPanel value={subAwgVpnLink} remark={`${client.email} — vpn://`} size={220} />}>
                        <Tooltip title={t('pages.clients.qrCode')}>
                          <Button size="small" icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
                        </Tooltip>
                      </Popover>
                    </div>
                  </div>
                )}
              </>
            )}

            {links.length > 0 && (
              <>
                <Divider>{t('pages.inbounds.copyLink')}</Divider>
                {links.map((link, idx) => {
                  const parts = parseLinkParts(link);
                  const fallback = `${t('pages.clients.link')} ${idx + 1}`;
                  const rowTitle = (parts && linkMetaText(parts)) || fallback;
                  const qrRemark = parts?.remark || rowTitle;
                  const canQr = !isPostQuantumLink(link);
                  return (
                    <div key={idx} className="link-row">
                      {parts
                        ? <LinkTags parts={parts} />
                        : <Tag className="link-row-tag">LINK</Tag>}
                      <span className="link-row-title" title={rowTitle}>{rowTitle}</span>
                      <div className="link-row-actions">
                        <Tooltip title={t('copy')}>
                          <Button size="small" icon={<CopyOutlined />} aria-label={t('copy')} onClick={() => copyValue(link)} />
                        </Tooltip>
                        {canQr && (
                          <Popover
                            trigger="click"
                            placement="left"
                            destroyOnHidden
                            content={<QrPanel value={link} remark={qrRemark} size={220} />}
                          >
                            <Tooltip title={t('pages.clients.qrCode')}>
                              <Button size="small" icon={<QrcodeOutlined />} aria-label={t('pages.clients.qrCode')} />
                            </Tooltip>
                          </Popover>
                        )}
                      </div>
                    </div>
                  );
                })}
              </>
            )}

            {wgConfigText && client && (
              <>
                <Divider>{t('pages.clients.wireguardConfig')}</Divider>
                <ConfigBlock
                  label={t('pages.clients.config')}
                  text={wgConfigText}
                  fileName={`${client.email}.conf`}
                  qrRemark={client.email || 'peer'}
                />
              </>
            )}
            {/* LUCX-HOOK: AWG — one .conf block per inbound (own ceiling + version select) */}
            {client && awgConfigs.map((cfg) => {
              const labelName = formatInboundLabel(cfg.ib.tag, cfg.ib.remark);
              return (
                <div key={cfg.ib.id}>
                  <Divider>
                    {t('pages.clients.awgConfig')}
                    {labelName ? ` — ${labelName}` : ''}
                  </Divider>
                  <Space style={{ width: '100%', justifyContent: 'flex-end', marginBottom: 8 }} align="center">
                    <Typography.Text type="secondary" style={{ fontSize: 12 }}>
                      {t('pages.clients.awgExportVersion')}
                    </Typography.Text>
                    <Select<AwgVersion>
                      size="small"
                      style={{ width: 180 }}
                      value={cfg.version}
                      onChange={(v) => setAwgExportById((prev) => ({ ...prev, [cfg.ib.id]: v }))}
                      options={[
                        { value: '1.5', label: t('pages.inbounds.form.awgVersion15'), disabled: !awgVersionAtLeast(cfg.ceiling, '1.5') },
                        { value: '2', label: t('pages.inbounds.form.awgVersion2'), disabled: !awgVersionAtLeast(cfg.ceiling, '2') },
                        { value: '3', label: t('pages.inbounds.form.awgVersion3'), disabled: !awgVersionAtLeast(cfg.ceiling, '3') },
                        { value: '3.1', label: t('pages.inbounds.form.awgVersion31'), disabled: !awgVersionAtLeast(cfg.ceiling, '3.1') },
                      ]}
                    />
                  </Space>
                  <ConfigBlock
                    label={t('pages.clients.config')}
                    text={cfg.text}
                    fileName={`${client.email}-awg${cfg.ib.id}.conf`}
                    qrRemark={client.email || 'peer'}
                  />
                </div>
              );
            })}
            {/* END LUCX-HOOK */}
          </>
        )}
      </Modal>

      <Modal
        open={ipsModalOpen}
        title={`${t('pages.inbounds.IPLimitlog')}${client?.email ? ` — ${client.email}` : ''}`}
        width={440}
        onCancel={() => setIpsModalOpen(false)}
        footer={[
          <Button key="refresh" icon={<ReloadOutlined />} loading={ipsLoading} onClick={loadIps}>
            {t('refresh')}
          </Button>,
          <Button key="clear" danger loading={ipsClearing} disabled={clientIps.length === 0} onClick={clearIps}>
            {t('pages.clients.clearAll')}
          </Button>,
          <Button key="close" type="primary" onClick={() => setIpsModalOpen(false)}>
            {t('close')}
          </Button>,
        ]}
      >
        {clientIps.length > 0 ? (
          <div style={{ maxHeight: 360, overflowY: 'auto' }}>
            {clientIps.map((entry, idx) => (
              <Tag
                key={idx}
                color="blue"
                style={{
                  display: 'block',
                  width: 'fit-content',
                  maxWidth: '100%',
                  marginBottom: 6,
                  padding: '2px 8px',
                  fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, monospace',
                }}
              >
                {entry.ip}{entry.time ? ` (${entry.time})` : ''}
                {entry.node ? (
                  <span style={{ marginInlineStart: 6, opacity: 0.85, fontWeight: 600 }}>@ {entry.node}</span>
                ) : null}
              </Tag>
            ))}
          </div>
        ) : (
          <Tag>{t('tgbot.noIpRecord')}</Tag>
        )}
      </Modal>
    </>
  );
}
