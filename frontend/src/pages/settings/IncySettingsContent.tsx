// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { Input, Select, Switch, Tabs } from 'antd';
import { useTranslation } from 'react-i18next';
import type { AllSetting } from '@/models/setting';
import { SettingListItem } from '@/components/ui';
import { remoteSourceBadge } from './subscriptionShared';

interface IncySettingsContentProps {
  allSetting: AllSetting;
  updateSetting: (patch: Partial<AllSetting>) => void;
  isMobile: boolean;
}

interface IncyConfig {
  enabled?: boolean;
  profileDescription?: string;
  sortOrder?: string;
  supportEmail?: string;
  announceUrl?: string;
  premiumUrl?: string;
  bannerText?: string;
  bannerButtonText?: string;
  bannerButtonUrl?: string;
  bannerBgColor?: string;
  bannerButtonColor?: string;
  hideUrl?: string;
  hideCheck?: string;
  perAppMode?: string;
  perAppList?: string;
  fragmentation?: {
    mode?: string;
    packets?: string;
    length?: string;
    interval?: string;
  };
  noises?: {
    mode?: string;
    type?: string;
    packet?: string;
    delay?: string;
  };
  resolve?: {
    mode?: string;
    dnsDomain?: string;
    dnsIp?: string;
  };
  noLimit?: boolean;
}

function parseConfig(raw: string): IncyConfig {
  if (!raw.trim()) return {};
  try {
    const value: unknown = JSON.parse(raw);
    return typeof value === 'object' && value !== null ? (value as IncyConfig) : {};
  } catch {
    return {};
  }
}

export default function IncySettingsContent({
  allSetting,
  updateSetting,
  isMobile,
}: IncySettingsContentProps) {
  const { t } = useTranslation();
  const config = parseConfig(allSetting.subIncyConfig);

  const updateConfig = (patch: Partial<IncyConfig>) => {
    updateSetting({ subIncyConfig: JSON.stringify({ ...config, ...patch }) });
  };

  const triStateOptions = [
    { value: '', label: t('pages.settings.subIncyUnchanged') },
    { value: 'on', label: t('enabled') },
    { value: 'off', label: t('disabled') },
  ];

  return (
    <>
      <SettingListItem
        paddings="small"
        title={t('pages.settings.subIncyAppManagement')}
        description={t('pages.settings.subIncyAppManagementDesc')}
      >
        <Switch
          checked={config.enabled === true}
          onChange={(enabled) => updateConfig({ enabled })}
        />
      </SettingListItem>

      <Tabs
        type="card"
        size="small"
        items={[
          {
            key: 'routing',
            label: isMobile ? t('pages.settings.subIncyRoutingShort') : t('pages.settings.subIncyRouting'),
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyEnableRouting')}
                  description={t('pages.settings.subIncyEnableRoutingDesc')}
                >
                  <Switch
                    checked={allSetting.subIncyEnableRouting}
                    onChange={(v) => updateSetting({ subIncyEnableRouting: v })}
                  />
                </SettingListItem>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyRoutingRules')}
                  badge={remoteSourceBadge(allSetting.subIncyRoutingRules)}
                  description={t('pages.settings.subIncyRoutingRulesDesc')}
                >
                  <Input.TextArea
                    value={allSetting.subIncyRoutingRules}
                    rows={4}
                    placeholder="incy://routing/onadd/... or https://.../DEFAULT.JSON"
                    onChange={(e) => updateSetting({ subIncyRoutingRules: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'profile',
            label: t('pages.settings.subIncyProfile'),
            children: (
              <>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyProfileDescription')}>
                  <Input
                    value={config.profileDescription ?? ''}
                    onChange={(e) => updateConfig({ profileDescription: e.target.value })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncySortOrder')}>
                  <Select
                    value={config.sortOrder ?? ''}
                    style={{ width: '100%' }}
                    onChange={(sortOrder) => updateConfig({ sortOrder })}
                    options={[
                      { value: '', label: t('pages.settings.subIncyUnchanged') },
                      { value: 'none', label: t('pages.settings.subIncySortNone') },
                      { value: 'ping', label: t('pages.settings.subIncySortPing') },
                      { value: 'name', label: t('pages.settings.subIncySortName') },
                    ]}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncySupportEmail')}>
                  <Input
                    value={config.supportEmail ?? ''}
                    placeholder="support@example.com"
                    onChange={(e) => updateConfig({ supportEmail: e.target.value })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyAnnounceUrl')}>
                  <Input
                    value={config.announceUrl ?? ''}
                    placeholder="https://example.com/news"
                    onChange={(e) => updateConfig({ announceUrl: e.target.value })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyPremiumUrl')}>
                  <Input
                    value={config.premiumUrl ?? ''}
                    placeholder="https://example.com/premium"
                    onChange={(e) => updateConfig({ premiumUrl: e.target.value })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyHideUrl')}>
                  <Select
                    value={config.hideUrl ?? ''}
                    style={{ width: '100%' }}
                    onChange={(hideUrl) => updateConfig({ hideUrl })}
                    options={triStateOptions}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyHideCheck')}>
                  <Select
                    value={config.hideCheck ?? ''}
                    style={{ width: '100%' }}
                    onChange={(hideCheck) => updateConfig({ hideCheck })}
                    options={triStateOptions}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyNoLimit')}>
                  <Switch
                    checked={config.noLimit === true}
                    onChange={(noLimit) => updateConfig({ noLimit })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'android',
            label: 'Android',
            children: (
              <>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyPerAppMode')}
                  description={t('pages.settings.subIncyPerAppModeDesc')}
                >
                  <Select
                    value={config.perAppMode ?? ''}
                    style={{ width: '100%' }}
                    onChange={(perAppMode) => updateConfig({ perAppMode })}
                    options={[
                      { value: '', label: t('pages.settings.subIncyUnchanged') },
                      { value: 'off', label: t('disabled') },
                      { value: 'proxy', label: t('pages.settings.subIncyPerAppProxy') },
                      { value: 'bypass', label: t('pages.settings.subIncyPerAppBypass') },
                    ]}
                  />
                </SettingListItem>
                <SettingListItem
                  paddings="small"
                  title={t('pages.settings.subIncyPerAppList')}
                  description={t('pages.settings.subIncyPerAppListDesc')}
                >
                  <Input.TextArea
                    rows={5}
                    value={config.perAppList ?? ''}
                    placeholder="org.telegram.messenger,com.google.android.youtube"
                    onChange={(e) => updateConfig({ perAppList: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
          {
            key: 'transport',
            label: t('pages.settings.subIncyTransport'),
            children: (
              <>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyFragmentation')}>
                  <Select
                    value={config.fragmentation?.mode ?? ''}
                    style={{ width: '100%' }}
                    onChange={(mode) =>
                      updateConfig({ fragmentation: { ...config.fragmentation, mode } })
                    }
                    options={triStateOptions}
                  />
                </SettingListItem>
                {config.fragmentation?.mode === 'on' ? (
                  <>
                    <SettingListItem paddings="small" title={t('pages.settings.subIncyFragmentPackets')}>
                      <Select
                        value={config.fragmentation?.packets ?? 'tlshello'}
                        style={{ width: '100%' }}
                        onChange={(packets) =>
                          updateConfig({ fragmentation: { ...config.fragmentation, packets } })
                        }
                        options={['tlshello', '1', '1-3', 'all'].map((value) => ({
                          value,
                          label: value,
                        }))}
                      />
                    </SettingListItem>
                    <SettingListItem paddings="small" title={t('pages.settings.subIncyFragmentLength')}>
                      <Input
                        value={config.fragmentation?.length ?? ''}
                        placeholder="10-30"
                        onChange={(e) =>
                          updateConfig({
                            fragmentation: { ...config.fragmentation, length: e.target.value },
                          })
                        }
                      />
                    </SettingListItem>
                    <SettingListItem paddings="small" title={t('pages.settings.subIncyFragmentInterval')}>
                      <Input
                        value={config.fragmentation?.interval ?? ''}
                        placeholder="10-30"
                        onChange={(e) =>
                          updateConfig({
                            fragmentation: { ...config.fragmentation, interval: e.target.value },
                          })
                        }
                      />
                    </SettingListItem>
                  </>
                ) : null}
                <SettingListItem paddings="small" title={t('pages.settings.subIncyNoises')}>
                  <Select
                    value={config.noises?.mode ?? ''}
                    style={{ width: '100%' }}
                    onChange={(mode) => updateConfig({ noises: { ...config.noises, mode } })}
                    options={triStateOptions}
                  />
                </SettingListItem>
                {config.noises?.mode === 'on' ? (
                  <>
                    <SettingListItem paddings="small" title={t('pages.settings.subIncyNoiseType')}>
                      <Select
                        value={config.noises?.type ?? 'rand'}
                        style={{ width: '100%' }}
                        onChange={(type) => updateConfig({ noises: { ...config.noises, type } })}
                        options={['rand', 'str', 'hex'].map((value) => ({
                          value,
                          label: value,
                        }))}
                      />
                    </SettingListItem>
                    <SettingListItem paddings="small" title={t('pages.settings.subIncyNoisePacket')}>
                      <Input
                        value={config.noises?.packet ?? ''}
                        placeholder="10-20"
                        onChange={(e) =>
                          updateConfig({ noises: { ...config.noises, packet: e.target.value } })
                        }
                      />
                    </SettingListItem>
                    <SettingListItem paddings="small" title={t('pages.settings.subIncyNoiseDelay')}>
                      <Input
                        value={config.noises?.delay ?? ''}
                        placeholder="10-50"
                        onChange={(e) =>
                          updateConfig({ noises: { ...config.noises, delay: e.target.value } })
                        }
                      />
                    </SettingListItem>
                  </>
                ) : null}
                <SettingListItem paddings="small" title={t('pages.settings.subIncyResolve')}>
                  <Select
                    value={config.resolve?.mode ?? ''}
                    style={{ width: '100%' }}
                    onChange={(mode) => updateConfig({ resolve: { ...config.resolve, mode } })}
                    options={triStateOptions}
                  />
                </SettingListItem>
                {config.resolve?.mode === 'on' ? (
                  <>
                    <SettingListItem paddings="small" title={t('pages.settings.subIncyResolveDomain')}>
                      <Input
                        value={config.resolve?.dnsDomain ?? ''}
                        placeholder="https://common.dot.dns.yandex.net/dns-query"
                        onChange={(e) =>
                          updateConfig({ resolve: { ...config.resolve, dnsDomain: e.target.value } })
                        }
                      />
                    </SettingListItem>
                    <SettingListItem paddings="small" title={t('pages.settings.subIncyResolveIp')}>
                      <Input
                        value={config.resolve?.dnsIp ?? ''}
                        placeholder="77.88.8.8"
                        onChange={(e) =>
                          updateConfig({ resolve: { ...config.resolve, dnsIp: e.target.value } })
                        }
                      />
                    </SettingListItem>
                  </>
                ) : null}
              </>
            ),
          },
          {
            key: 'banner',
            label: t('pages.settings.subIncyBanner'),
            children: (
              <>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyBannerText')}>
                  <Input
                    value={config.bannerText ?? ''}
                    onChange={(e) => updateConfig({ bannerText: e.target.value })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyBannerButtonText')}>
                  <Input
                    value={config.bannerButtonText ?? ''}
                    onChange={(e) => updateConfig({ bannerButtonText: e.target.value })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyBannerButtonUrl')}>
                  <Input
                    value={config.bannerButtonUrl ?? ''}
                    placeholder="https://example.com/promo"
                    onChange={(e) => updateConfig({ bannerButtonUrl: e.target.value })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyBannerBgColor')}>
                  <Input
                    value={config.bannerBgColor ?? ''}
                    placeholder="#E53E3E"
                    onChange={(e) => updateConfig({ bannerBgColor: e.target.value })}
                  />
                </SettingListItem>
                <SettingListItem paddings="small" title={t('pages.settings.subIncyBannerButtonColor')}>
                  <Input
                    value={config.bannerButtonColor ?? ''}
                    placeholder="#38A169"
                    onChange={(e) => updateConfig({ bannerButtonColor: e.target.value })}
                  />
                </SettingListItem>
              </>
            ),
          },
        ]}
      />
    </>
  );
}
