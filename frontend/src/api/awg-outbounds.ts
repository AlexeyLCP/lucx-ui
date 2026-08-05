// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import {
  AwgOutboundRowSchema,
  AwgOutboundSchema,
  AwgOutboundSettingsSchema,
  AwgOutboundStatusSchema,
  AwgOutboundTestSchema,
  type AwgOutbound,
  type AwgOutboundRow,
  type AwgOutboundSettings,
  type AwgOutboundStatus,
  type AwgOutboundTest,
} from '@/schemas/awg-outbound';

export type { AwgOutbound, AwgOutboundRow, AwgOutboundSettings, AwgOutboundStatus, AwgOutboundTest };

// API client for the 8 AWG-outbound REST endpoints registered in
// internal/web/controller/awg_outbound.go (Task 6). Routes live under
// /panel/api/awg-outbounds/*. Each method returns the project's Msg<T>
// envelope (success/msg/obj) so callers can branch on msg.success and read
// msg.obj the same way they do for nodes/hosts. Responses are validated with
// parseMsg so a drifted backend shape fails loudly in the console rather than
// silently rendering garbage in the UI (Task 9 builds the components on top).
const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

const BASE = '/panel/api/awg-outbounds';

export const awgOutboundsApi = {
  list: async (): Promise<Msg<AwgOutboundRow[]>> => {
    const raw = await HttpUtil.get<AwgOutboundRow[]>(`${BASE}/list`, undefined, { silent: true });
    return parseMsg(raw, z.array(AwgOutboundRowSchema), 'awg-outbounds/list');
  },
  add: async (data: Partial<AwgOutbound>): Promise<Msg<AwgOutbound>> => {
    const raw = await HttpUtil.post<AwgOutbound>(`${BASE}/add`, data, JSON_HEADERS);
    return parseMsg(raw, AwgOutboundSchema, 'awg-outbounds/add');
  },
  del: (id: number): Promise<Msg<null>> => HttpUtil.post<null>(`${BASE}/del/${id}`, {}),
  update: async (data: AwgOutbound): Promise<Msg<AwgOutbound>> => {
    const raw = await HttpUtil.post<AwgOutbound>(`${BASE}/update/${data.id}`, data, JSON_HEADERS);
    return parseMsg(raw, AwgOutboundSchema, 'awg-outbounds/update');
  },
  // JSON_HEADERS is load-bearing: http-init serializes the body as JSON only
  // when Content-Type is application/json, otherwise it falls back to
  // form-urlencoded (`enable=true`). The backend binds this endpoint with
  // ShouldBindJSON, so a form body fails to parse and Enable silently defaults
  // to false — turning every "enable" into a "disable" (lucx.69). add/update/
  // parseConf already pass JSON_HEADERS for the same reason.
  enable: (id: number, enable: boolean): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${BASE}/enable/${id}`, { enable }, JSON_HEADERS),
  status: async (id: number): Promise<Msg<AwgOutboundStatus>> => {
    const raw = await HttpUtil.get<AwgOutboundStatus>(`${BASE}/status/${id}`, undefined, { silent: true });
    return parseMsg(raw, AwgOutboundStatusSchema, 'awg-outbounds/status');
  },
  test: async (id: number): Promise<Msg<AwgOutboundTest>> => {
    const raw = await HttpUtil.post<AwgOutboundTest>(`${BASE}/test/${id}`, {});
    return parseMsg(raw, AwgOutboundTestSchema, 'awg-outbounds/test');
  },
  parseConf: async (conf: string): Promise<Msg<AwgOutboundSettings>> => {
    const raw = await HttpUtil.post<AwgOutboundSettings>(`${BASE}/parseConf`, { conf }, JSON_HEADERS);
    return parseMsg(raw, AwgOutboundSettingsSchema, 'awg-outbounds/parseConf');
  },
};