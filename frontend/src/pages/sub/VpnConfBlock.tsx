// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { useEffect, useState } from 'react';

import ConfigBlock from '@/components/clients/ConfigBlock';
import { vpnConfFromLink } from '@/lib/awg/vpnuri';

export default function VpnConfBlock({
  link,
  rowTitle,
  label,
}: {
  link: string;
  rowTitle: string;
  label: string;
}) {
  const [text, setText] = useState('');
  useEffect(() => {
    let cancelled = false;
    void vpnConfFromLink(link).then((conf) => {
      if (!cancelled && conf) setText(conf);
    });
    return () => {
      cancelled = true;
    };
  }, [link]);
  if (!text) return null;
  const remark = /^#\s?(.*)$/m.exec(text)?.[1]?.trim() || rowTitle;
  return (
    <ConfigBlock
      label={label}
      text={text}
      fileName={`${remark || 'peer'}.conf`}
      qrRemark={remark}
      tagColor="purple"
    />
  );
}
