// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { Tag, Tooltip } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined, WarningOutlined } from '@ant-design/icons';

export interface AwgOutboundStatusInfo {
  up: boolean;
  fallback: boolean;
  handshakeAge?: string;
  rx?: number;
  tx?: number;
}

// parseStatusString turns the controller's human-readable status string
// (e.g. "up; handshake 45s ago; rx=12345 tx=6789" or
// "down (fallback active — default route)") into the structured shape the
// badge renders. The tab calls this so the badge stays a pure presentation
// component.
export function parseAwgOutboundStatus(status: string): AwgOutboundStatusInfo {
  const s = status ?? '';
  const up = s.startsWith('up');
  const fallback = s.includes('fallback');
  if (up) {
    const handshake = /handshake\s+([^;]+)/i.exec(s);
    const rx = /rx[=\s](\d+)/i.exec(s);
    const tx = /tx[=\s](\d+)/i.exec(s);
    return {
      up,
      fallback,
      handshakeAge: handshake?.[1] ?? '',
      rx: rx ? Number(rx[1]) : undefined,
      tx: tx ? Number(tx[1]) : undefined,
    };
  }
  return { up: false, fallback };
}

export function AwgOutboundStatusBadge({ status }: { status: AwgOutboundStatusInfo }) {
  if (status.up) {
    const tip = `handshake ${status.handshakeAge ?? ''} ago; rx ${status.rx ?? 0} tx ${status.tx ?? 0}`.trim();
    return (
      <Tooltip title={tip}>
        <Tag icon={<CheckCircleOutlined />} color="success">
          Up
        </Tag>
      </Tooltip>
    );
  }
  return (
    <Tooltip title="interface down — traffic falls back to the default route (bypasses VPN)">
      <Tag
        icon={status.fallback ? <WarningOutlined /> : <CloseCircleOutlined />}
        color={status.fallback ? 'warning' : 'error'}
      >
        {status.fallback ? 'Down (fallback active — WARNING)' : 'Down'}
      </Tag>
    </Tooltip>
  );
}