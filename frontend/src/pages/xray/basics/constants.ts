export const ROUTING_DOMAIN_STRATEGIES = ['AsIs', 'IPIfNonMatch', 'IPOnDemand'];
export const LOG_LEVELS = ['none', 'debug', 'info', 'warning', 'error'];
export const ACCESS_LOG = ['none', './access.log'];
export const ERROR_LOG = ['none', './error.log'];
export const MASK_ADDRESS = ['quarter', 'half', 'full'];
export const BITTORRENT_PROTOCOLS = ['bittorrent'];

export const IPS_OPTIONS = [
  { label: 'Private IPs', value: 'geoip:private' },
  { label: '🇮🇷 Iran', value: 'ext:geoip_IR.dat:ir' },
  { label: '🇨🇳 China', value: 'geoip:cn' },
  { label: '🇷🇺 Russia', value: 'ext:geoip_RU.dat:ru' },
  // LUCX-HOOK: RoscomVPN geoip (direct/whitelist CIDRs)
  { label: 'RoscomVPN direct', value: 'ext:geoip_ROSCOM.dat:direct' },
  { label: 'RoscomVPN whitelist', value: 'ext:geoip_ROSCOM.dat:whitelist' },
  // END LUCX-HOOK
  { label: '🇻🇳 Vietnam', value: 'geoip:vn' },
  { label: '🇪🇸 Spain', value: 'geoip:es' },
  { label: '🇮🇩 Indonesia', value: 'geoip:id' },
  { label: '🇺🇦 Ukraine', value: 'geoip:ua' },
  { label: '🇹🇷 Türkiye', value: 'geoip:tr' },
  { label: '🇧🇷 Brazil', value: 'geoip:br' },
];
export const DOMAINS_OPTIONS = [
  { label: '🇮🇷 Iran', value: 'ext:geosite_IR.dat:ir' },
  { label: '🇮🇷 .ir', value: 'regexp:.*\\.ir$' },
  { label: '🇮🇷 .ایران', value: 'regexp:.*\\.xn--mgba3a4f16a$' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇨🇳 .cn', value: 'regexp:.*\\.cn$' },
  { label: '🇷🇺 Russia', value: 'ext:geosite_RU.dat:ru-available-only-inside' },
  // LUCX-HOOK: RoscomVPN geosite (RKN geoblock + RU category)
  { label: 'RoscomVPN RU', value: 'ext:geosite_ROSCOM.dat:category-ru' },
  { label: 'RoscomVPN geoblock RU', value: 'ext:geosite_ROSCOM.dat:category-geoblock-ru' },
  // END LUCX-HOOK
  { label: '🇷🇺 .ru', value: 'regexp:.*\\.ru$' },
  { label: '🇷🇺 .su', value: 'regexp:.*\\.su$' },
  { label: '🇷🇺 .рф', value: 'regexp:.*\\.xn--p1ai$' },
  { label: '🇻🇳 .vn', value: 'regexp:.*\\.vn$' },
];
export const BLOCK_DOMAINS_OPTIONS = [
  { label: 'Ads All', value: 'geosite:category-ads-all' },
  { label: 'Ads IR 🇮🇷', value: 'ext:geosite_IR.dat:category-ads-all' },
  { label: 'Ads RU 🇷🇺', value: 'ext:geosite_RU.dat:category-ads-all' },
  // LUCX-HOOK: RoscomVPN ads + geoblock
  { label: 'Ads RoscomVPN', value: 'ext:geosite_ROSCOM.dat:category-ads' },
  { label: 'RoscomVPN geoblock RU', value: 'ext:geosite_ROSCOM.dat:category-geoblock-ru' },
  // END LUCX-HOOK
  { label: 'Malware 🇮🇷', value: 'ext:geosite_IR.dat:malware' },
  { label: 'Phishing 🇮🇷', value: 'ext:geosite_IR.dat:phishing' },
  { label: 'Cryptominers 🇮🇷', value: 'ext:geosite_IR.dat:cryptominers' },
  { label: 'Adult +18', value: 'geosite:category-porn' },
  { label: '🇮🇷 Iran', value: 'ext:geosite_IR.dat:ir' },
  { label: '🇮🇷 .ir', value: 'regexp:.*\\.ir$' },
  { label: '🇮🇷 .ایران', value: 'regexp:.*\\.xn--mgba3a4f16a$' },
  { label: '🇨🇳 China', value: 'geosite:cn' },
  { label: '🇨🇳 .cn', value: 'regexp:.*\\.cn$' },
  { label: '🇷🇺 Russia', value: 'ext:geosite_RU.dat:ru-available-only-inside' },
  { label: '🇷🇺 .ru', value: 'regexp:.*\\.ru$' },
  { label: '🇷🇺 .su', value: 'regexp:.*\\.su$' },
  { label: '🇷🇺 .рф', value: 'regexp:.*\\.xn--p1ai$' },
  { label: '🇻🇳 .vn', value: 'regexp:.*\\.vn$' },
];
export const SERVICES_OPTIONS = [
  { label: 'Apple', value: 'geosite:apple' },
  { label: 'Meta', value: 'geosite:meta' },
  { label: 'Google', value: 'geosite:google' },
  { label: 'OpenAI', value: 'geosite:openai' },
  { label: 'Spotify', value: 'geosite:spotify' },
  { label: 'Netflix', value: 'geosite:netflix' },
  { label: 'Reddit', value: 'geosite:reddit' },
  { label: 'Speedtest', value: 'geosite:speedtest' },
  // LUCX-HOOK: RoscomVPN service lists
  { label: 'YouTube (RoscomVPN)', value: 'ext:geosite_ROSCOM.dat:youtube' },
  { label: 'Telegram (RoscomVPN)', value: 'ext:geosite_ROSCOM.dat:telegram' },
  { label: 'Steam (RoscomVPN)', value: 'ext:geosite_ROSCOM.dat:steam' },
  // END LUCX-HOOK
];

export const directSettings = { tag: 'direct', protocol: 'freedom' };
export const blockedSettings = { tag: 'blocked', protocol: 'blackhole', settings: {} };
export const ipv4Settings = {
  tag: 'IPv4',
  protocol: 'freedom',
  settings: { domainStrategy: 'UseIPv4' },
};
