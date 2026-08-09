// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import {
  NaiveConfigSchema,
  NaiveStatusSchema,
  type NaiveConfig,
  type NaiveStatus,
} from '@/schemas/tunnel';

export type { NaiveConfig, NaiveStatus };

// API client for the tunnel sidecar endpoints registered in
// internal/web/controller/tunnel.go. Routes live under
// /panel/api/tunnel/naive/*. JSON_HEADERS is load-bearing on every POST:
// http-init serializes the body as JSON only when Content-Type is
// application/json, otherwise the backend ShouldBindJSON sees a form body
// and fields silently default (lucx.69 lesson).
const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

const BASE = '/panel/api/tunnel/naive';

export const tunnelsApi = {
  status: async (): Promise<Msg<NaiveStatus>> => {
    const raw = await HttpUtil.get<NaiveStatus>(`${BASE}/status`, undefined, { silent: true });
    return parseMsg(raw, NaiveStatusSchema, 'tunnel/status');
  },
  config: async (): Promise<Msg<NaiveConfig>> => {
    const raw = await HttpUtil.get<NaiveConfig>(`${BASE}/config`, undefined, { silent: true });
    return parseMsg(raw, NaiveConfigSchema, 'tunnel/config');
  },
  saveConfig: async (cfg: NaiveConfig): Promise<Msg<NaiveStatus>> => {
    const raw = await HttpUtil.post<NaiveStatus>(`${BASE}/config`, cfg, JSON_HEADERS);
    return parseMsg(raw, NaiveStatusSchema, 'tunnel/saveConfig');
  },
  start: (): Promise<Msg<null>> => HttpUtil.post<null>(`${BASE}/start`, {}, JSON_HEADERS),
  stop: (): Promise<Msg<null>> => HttpUtil.post<null>(`${BASE}/stop`, {}, JSON_HEADERS),
  restart: (): Promise<Msg<null>> => HttpUtil.post<null>(`${BASE}/restart`, {}, JSON_HEADERS),
  logs: (lines = 200): Promise<Msg<string[]>> =>
    HttpUtil.get<string[]>(`${BASE}/logs?lines=${lines}`),
  preview: async (cfg: NaiveConfig): Promise<Msg<{ caddyfile: string }>> => {
    const raw = await HttpUtil.post<{ caddyfile: string }>(`${BASE}/preview`, cfg, JSON_HEADERS);
    return parseMsg(raw, z.object({ caddyfile: z.string() }), 'tunnel/preview');
  },
  validate: async (text: string): Promise<Msg<{ valid: boolean }>> => {
    const raw = await HttpUtil.post<{ valid: boolean }>(`${BASE}/validate`, { text }, JSON_HEADERS);
    return parseMsg(raw, z.object({ valid: z.boolean() }), 'tunnel/validate');
  },
  download: (url: string): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${BASE}/download`, { url }, JSON_HEADERS),
  upload: (file: File): Promise<Msg<null>> => {
    const fd = new FormData();
    fd.append('file', file);
    return HttpUtil.post<null>(`${BASE}/upload`, fd);
  },
  deleteBinary: (): Promise<Msg<null>> => HttpUtil.post<null>(`${BASE}/deleteBinary`, {}, JSON_HEADERS),
};
