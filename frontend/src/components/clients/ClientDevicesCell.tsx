// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useState } from 'react';
import type { ReactNode } from 'react';
import { Divider, Drawer, Popover, Spin, Tag, Typography } from 'antd';
import { useTranslation } from 'react-i18next';
import { HttpUtil, IntlUtil } from '@/utils';
import { useDatepicker } from '@/hooks/useDatepicker';
import { useCoarsePointer } from '@/hooks/useCoarsePointer';
import { useMediaQuery } from '@/hooks/useMediaQuery';
import { useBodyScrollLock } from '@/hooks/useBodyScrollLock';
import { normalizeClientHwids, type ClientHwidInfo } from '@/lib/clients/hwid-log';

// One inbound a client is using right now, as attributed by the server
// (traffic job): busiest first.
export type LiveInbound = {
  tag: string;
  remark: string;
  protocol: string;
};

// A client address seen in the last few seconds, labelled with its provider
// from the server's local IP databases. Already masked by the server when the
// Xray log "Mask Address" option is on.
export type LiveIP = {
  ip: string;
  lastSeen: number;
  inbounds?: string[];
} & IPInfo;

// Location and network of an IP, from local MMDB files on the server (MaxMind
// GeoLite2 or DB-IP Lite). Any field can be missing when no database covers it.
type IPInfo = {
  asn?: number;
  org?: string;
  country?: string;
  countryNames?: Record<string, string>;
  city?: string;
  // Databases used, comma separated: "maxmind", "dbip".
  source?: string;
};

// The "Devices" cell: registered HWID devices against the effective limit
// (0 = unlimited) plus the addresses connected now.
export type ClientDevices = {
  count: number;
  limit: number;
  ips?: LiveIP[];
};

interface ClientDevicesCellProps {
  email: string;
  devices: ClientDevices | undefined;
  inbounds: LiveInbound[] | undefined;
}

// Device lists are fetched on first hover and reused for a minute, so moving
// the mouse across the table doesn't fire a request per row per pass.
const HWID_CACHE_MS = 60_000;
const hwidCache = new Map<string, { at: number; list: ClientHwidInfo[] }>();

async function loadHwids(email: string): Promise<ClientHwidInfo[]> {
  const cached = hwidCache.get(email);
  if (cached && Date.now() - cached.at < HWID_CACHE_MS) return cached.list;
  const msg = (await HttpUtil.post(
    `/panel/api/clients/hwids/${encodeURIComponent(email)}`,
    undefined,
    {
      silent: true,
    },
  )) as { success?: boolean; obj?: unknown };
  const list = msg?.success ? normalizeClientHwids(msg.obj) : [];
  list.sort((a, b) => b.lastSeen - a.lastSeen);
  hwidCache.set(email, { at: Date.now(), list });
  return list;
}

// Device times read day/month/year with the time, whatever the browser
// locale; a Jalali-calendar panel keeps the panel's own formatter.
function formatSeen(ts: number, calendar: string): string {
  if (calendar === 'jalalian') return IntlUtil.formatDate(ts, 'jalalian');
  const d = new Date(ts);
  if (!isFinite(d.getTime())) return '';
  const p = (n: number) => String(n).padStart(2, '0');
  return `${p(d.getDate())}/${p(d.getMonth() + 1)}/${d.getFullYear()}, ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}`;
}

// relativeDay returns "today" / "yesterday" (localized) for recent dates.
function relativeDay(ts: number, language: string): string {
  const day = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();
  const diff = Math.round((day(new Date()) - day(new Date(ts))) / 86_400_000);
  if (diff !== 0 && diff !== 1) return '';
  try {
    return new Intl.RelativeTimeFormat(language, { numeric: 'auto' }).format(-diff, 'day');
  } catch {
    return '';
  }
}

// flag turns an ISO country code into its emoji flag.
function flag(iso?: string): string {
  if (!iso || !/^[A-Za-z]{2}$/.test(iso)) return '';
  return String.fromCodePoint(...[...iso.toUpperCase()].map((c) => 0x1f1a5 + c.charCodeAt(0)));
}

function geoLabel(ip: IPInfo, language: string): string {
  const base = language.split('-')[0];
  const country =
    ip.countryNames?.[language] ||
    ip.countryNames?.[base] ||
    ip.countryNames?.en ||
    ip.country ||
    '';
  const place = [ip.city, country].filter(Boolean).join(', ');
  return place ? `${flag(ip.country)} ${place}`.trim() : '';
}

function IPLine({
  ip,
  info,
  language,
  extra,
}: {
  ip: string;
  info: IPInfo;
  language: string;
  extra?: ReactNode;
}) {
  const geo = geoLabel(info, language);
  const provider = providerLabel(info);
  return (
    <div style={{ display: 'grid', gap: 2, minWidth: 0 }}>
      <div style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: 4 }}>
        <Typography.Text
          code
          copyable={{ text: ip }}
          style={{ fontSize: 12, wordBreak: 'break-all' }}
        >
          {ip}
        </Typography.Text>
        {extra}
      </div>
      {geo && <Typography.Text style={{ fontSize: 12 }}>{geo}</Typography.Text>}
      {provider && (
        <Typography.Text type="secondary" style={{ fontSize: 12 }}>
          {provider}
        </Typography.Text>
      )}
    </div>
  );
}

function InboundTags({ names }: { names?: string[] }) {
  if (!names?.length) return null;
  return (
    <>
      {names.map((n) => (
        <Tag
          key={n}
          color="geekblue"
          style={{ marginInlineEnd: 0, fontSize: 11, lineHeight: '18px', paddingInline: 6 }}
        >
          {n}
        </Tag>
      ))}
    </>
  );
}

// osLabel joins OS name and version without repeating the name when the app
// already put it in the version ("Android" + "Android 17").
function osLabel(os?: string, version?: string): string {
  const o = (os || '').trim();
  const v = (version || '').trim();
  if (o && v.toLowerCase().startsWith(o.toLowerCase())) return v;
  return [o, v].filter(Boolean).join(' ');
}

function providerLabel(ip: { asn?: number; org?: string }): string {
  return [ip.asn ? `AS${ip.asn}` : '', ip.org || ''].filter(Boolean).join(' · ');
}

function LiveTags({ ip, label }: { ip: LiveIP | undefined; label: string }) {
  if (!ip) return null;
  return (
    <>
      <Tag color="green" style={{ marginInlineEnd: 0, fontSize: 11, lineHeight: '18px' }}>
        {label}
      </Tag>
      <InboundTags names={ip.inbounds} />
    </>
  );
}

function DevicesDetail({
  email,
  open,
  ips,
  scroll = true,
}: {
  email: string;
  open: boolean;
  ips: LiveIP[];
  scroll?: boolean;
}) {
  const { t, i18n } = useTranslation();
  const liveIPs = new Map(ips.map((ip) => [ip.ip, ip]));
  const { datepicker } = useDatepicker();
  const [hwids, setHwids] = useState<ClientHwidInfo[] | null>(null);

  useEffect(() => {
    if (!open) return;
    let alive = true;
    loadHwids(email)
      .then((list) => alive && setHwids(list))
      .catch(() => alive && setHwids([]));
    return () => {
      alive = false;
    };
  }, [email, open]);

  if (hwids === null) return <Spin size="small" />;

  // A device's own IP is the one its app used for the last subscription
  // fetch. With a single registered device, every live address is that
  // device; otherwise live addresses no device matches are listed apart.
  const single = hwids.length === 1;
  const claimed = new Set(hwids.map((d) => d.lastIp).filter(Boolean) as string[]);
  const orphanIPs = single ? [] : ips.filter((ip) => !claimed.has(ip.ip));
  const sources = new Set([...ips, ...hwids].flatMap((x) => (x.source ? x.source.split(',') : [])));
  const connected = t('pages.clients.connectedNow');

  return (
    <div
      style={{
        display: 'grid',
        gap: 12,
        width: '100%',
        ...(scroll
          ? { maxHeight: 'min(60vh, 60dvh)', overflowY: 'auto' as const, paddingRight: 4 }
          : {}),
      }}
    >
      {hwids.length === 0 && (
        <Typography.Text type="secondary">{t('pages.clients.noHwids')}</Typography.Text>
      )}
      {hwids.map((d, idx) => {
        const name = [d.deviceModel, osLabel(d.deviceOs, d.osVersion)].filter(Boolean).join(' · ');
        const ownLive = d.lastIp ? liveIPs.get(d.lastIp) : undefined;
        const extraLive = single ? ips.filter((ip) => ip.ip !== d.lastIp) : [];
        return (
          <div key={d.id} style={{ display: 'grid', gap: 4 }}>
            {idx > 0 && <Divider style={{ margin: '0 0 6px' }} />}
            <Typography.Text strong style={{ wordBreak: 'break-word' }}>
              {name || d.userAgent || t('pages.clients.hwidDevice')}
            </Typography.Text>
            {d.fingerprint && (
              <Typography.Text type="secondary" code style={{ fontSize: 12 }}>
                {t('pages.clients.hwidFingerprint')}: {d.fingerprint}
              </Typography.Text>
            )}
            {d.lastIp && (
              <IPLine
                ip={d.lastIp}
                info={d}
                language={i18n.language}
                extra={<LiveTags ip={ownLive} label={connected} />}
              />
            )}
            {extraLive.map((ip) => (
              <IPLine
                key={ip.ip}
                ip={ip.ip}
                info={ip}
                language={i18n.language}
                extra={<LiveTags ip={ip} label={connected} />}
              />
            ))}
            <Typography.Text type="secondary" style={{ fontSize: 12 }}>
              {t('pages.clients.lastSeen')}: {formatSeen(d.lastSeen, datepicker)}
              {relativeDay(d.lastSeen, i18n.language) &&
                ` (${relativeDay(d.lastSeen, i18n.language)})`}
            </Typography.Text>
          </div>
        );
      })}
      {orphanIPs.length > 0 && (
        <div style={{ display: 'grid', gap: 6 }}>
          <Divider style={{ margin: 0 }} />
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {connected}
          </Typography.Text>
          {orphanIPs.map((ip) => (
            <IPLine
              key={ip.ip}
              ip={ip.ip}
              info={ip}
              language={i18n.language}
              extra={<InboundTags names={ip.inbounds} />}
            />
          ))}
        </div>
      )}
      {/* GeoLite2's licence asks for this credit wherever its data is shown. */}
      {sources.has('maxmind') && (
        <Typography.Text type="secondary" style={{ fontSize: 11 }}>
          This product includes GeoLite2 data created by MaxMind, available from{' '}
          <a href="https://www.maxmind.com" target="_blank" rel="noopener noreferrer">
            maxmind.com
          </a>
        </Typography.Text>
      )}
    </div>
  );
}

// ClientDevicesCell shows "registered/limit" devices, the inbounds the client
// is on right now under it, and on hover the live IPs (with provider) and the
// registered HWID devices.
export default function ClientDevicesCell({ email, devices, inbounds }: ClientDevicesCellProps) {
  const { t } = useTranslation();
  // A sheet on touch screens and on anything narrower than a small laptop,
  // where a floating card has no room; the hover card only on wide desktops.
  const coarse = useCoarsePointer();
  const { isMobile: narrow } = useMediaQuery(1024);
  const touch = coarse || narrow;
  const [open, setOpen] = useState(false);
  // The sheet must not scroll the page behind it, however short its list.
  useBodyScrollLock(touch && open);
  const count = devices?.count ?? 0;
  const limit = devices?.limit ?? 0;
  const full = limit > 0 && count >= limit;
  const ips = devices?.ips ?? [];
  const countLabel = limit > 0 ? `${count}/${limit}` : `${count} / ${t('unlimited')}`;

  const summaryStyle = {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: 4,
  } as const;
  const summaryContent = (
    <>
      <Tag
        color={full ? 'red' : count > 0 ? 'blue' : 'default'}
        style={{
          marginInlineEnd: 0,
          minWidth: 48,
          textAlign: 'center',
          fontVariantNumeric: 'tabular-nums',
        }}
      >
        {countLabel}
      </Tag>
      {inbounds && inbounds.length > 0 && (
        <div style={{ display: 'flex', flexWrap: 'wrap', justifyContent: 'center', gap: 4 }}>
          {inbounds.map((ib, i) => (
            <Tag
              key={ib.tag}
              color={i === 0 ? 'geekblue' : 'default'}
              style={{ marginInlineEnd: 0, fontSize: 11, lineHeight: '18px', paddingInline: 6 }}
            >
              {ib.remark}
            </Tag>
          ))}
        </div>
      )}
    </>
  );

  // Phones and tablets have no hover and little room for a floating card:
  // tapping opens a full-width bottom sheet that scrolls on its own and stays
  // clear of Safari's toolbar (dvh + safe-area inset).
  if (touch) {
    return (
      <>
        <div
          role="button"
          tabIndex={0}
          aria-label={`${t('pages.clients.devices')}: ${countLabel}`}
          onClick={() => setOpen(true)}
          onKeyDown={(e) => {
            if (e.key === 'Enter' || e.key === ' ') {
              e.preventDefault();
              setOpen(true);
            }
          }}
          style={{ ...summaryStyle, cursor: 'pointer' }}
        >
          {summaryContent}
        </div>
        <Drawer
          open={open}
          onClose={() => setOpen(false)}
          placement="bottom"
          destroyOnHidden
          title={`${t('pages.clients.devices')} · ${email} · ${countLabel}`}
          styles={{
            wrapper: { height: 'auto', maxHeight: 'min(85vh, 85dvh)' },
            // The body is its own scroll area with a bounded height, so a
            // swipe scrolls the device list, never the page behind it.
            body: {
              maxHeight: 'calc(min(85vh, 85dvh) - 64px)',
              overflowY: 'auto',
              overscrollBehavior: 'contain',
              touchAction: 'pan-y',
              paddingBottom: 'calc(16px + env(safe-area-inset-bottom))',
            },
          }}
        >
          <DevicesDetail email={email} open={open} ips={ips} scroll={false} />
        </Drawer>
      </>
    );
  }

  return (
    <Popover
      trigger="hover"
      mouseEnterDelay={0.25}
      open={open}
      onOpenChange={setOpen}
      placement="left"
      autoAdjustOverflow
      styles={{ root: { width: 'min(360px, calc(100vw - 24px))' } }}
      title={t('pages.clients.devices')}
      content={<DevicesDetail email={email} open={open} ips={ips} />}
    >
      <div style={{ ...summaryStyle, cursor: 'help' }}>{summaryContent}</div>
    </Popover>
  );
}
