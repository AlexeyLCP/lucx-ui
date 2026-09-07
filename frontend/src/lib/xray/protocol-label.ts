export function protocolLabel(protocol: string, t: (key: string) => string): string {
  if (protocol === 'awg') return t('pages.inbounds.protocolNames.awg').toLowerCase();
  if (protocol === 'amneziawg') return t('pages.inbounds.protocolNames.amneziawg').toLowerCase();
  if (protocol === 'anytls') return t('pages.inbounds.protocolNames.anytls').toLowerCase();
  if (protocol === 'tproxy') return t('pages.inbounds.protocolNames.tproxy').toLowerCase();
  if (protocol === 'cover') return t('pages.inbounds.protocolNames.cover').toLowerCase();
  return protocol;
}
