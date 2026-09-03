export function protocolLabel(protocol: string, t: (key: string) => string): string {
  if (protocol === 'awg') return t('pages.inbounds.protocolNames.awg');
  if (protocol === 'amneziawg') return t('pages.inbounds.protocolNames.amneziawg');
  if (protocol === 'anytls') return t('pages.inbounds.protocolNames.anytls');
  if (protocol === 'tproxy') return t('pages.inbounds.protocolNames.tproxy');
  return protocol;
}
