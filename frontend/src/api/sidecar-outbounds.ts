// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import {
  SidecarOutboundRowSchema,
  SidecarOutboundSchema,
  SidecarBinariesSchema,
  SidecarParseLinkSchema,
  SidecarTestSchema,
  type SidecarOutbound,
  type SidecarOutboundRow,
  type SidecarProtocol,
} from '@/schemas/sidecar-outbound';

export type { SidecarOutbound, SidecarOutboundRow, SidecarProtocol };

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };
const BASE = '/panel/api/sidecar-outbounds';

export const sidecarOutboundsApi = {
  list: async (): Promise<Msg<SidecarOutboundRow[]>> => {
    const raw = await HttpUtil.get<SidecarOutboundRow[]>(`${BASE}/list`, undefined, {
      silent: true,
    });
    return parseMsg(raw, z.array(SidecarOutboundRowSchema), 'sidecar-outbounds/list');
  },
  add: async (data: Partial<SidecarOutbound>): Promise<Msg<SidecarOutbound>> => {
    const raw = await HttpUtil.post<SidecarOutbound>(`${BASE}/add`, data, JSON_HEADERS);
    return parseMsg(raw, SidecarOutboundSchema, 'sidecar-outbounds/add');
  },
  del: (id: number): Promise<Msg<null>> => HttpUtil.post<null>(`${BASE}/del/${id}`, {}),
  update: async (data: SidecarOutbound): Promise<Msg<SidecarOutbound>> => {
    const raw = await HttpUtil.post<SidecarOutbound>(
      `${BASE}/update/${data.id}`,
      data,
      JSON_HEADERS,
    );
    return parseMsg(raw, SidecarOutboundSchema, 'sidecar-outbounds/update');
  },
  enable: (id: number, enable: boolean): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${BASE}/enable/${id}`, { enable }, JSON_HEADERS),
  test: async (id: number): Promise<Msg<{ ok?: boolean; latency_ms?: number; raw?: string }>> => {
    const raw = await HttpUtil.post(`${BASE}/test/${id}`, {});
    return parseMsg(raw, SidecarTestSchema, 'sidecar-outbounds/test');
  },
  parseLink: async (link: string) => {
    const raw = await HttpUtil.post(`${BASE}/parseLink`, { link }, JSON_HEADERS);
    return parseMsg(raw, SidecarParseLinkSchema, 'sidecar-outbounds/parseLink');
  },
  upload: (protocol: SidecarProtocol, file: File): Promise<unknown> => {
    const fd = new FormData();
    fd.append('file', file);
    return HttpUtil.post(`${BASE}/upload/${protocol}`, fd);
  },
  binaries: async () => {
    const raw = await HttpUtil.get(`${BASE}/binaries`, undefined, { silent: true });
    return parseMsg(raw, SidecarBinariesSchema, 'sidecar-outbounds/binaries');
  },
  deleteBinary: (protocol: SidecarProtocol): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${BASE}/deleteBinary/${protocol}`, {}),
};
