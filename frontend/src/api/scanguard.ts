// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import {
  ScanguardStatusSchema,
  type ScanguardConfig,
  type ScanguardStatus,
} from '@/schemas/scanguard';

// JSON_HEADERS is load-bearing on every POST (lucx.69 lesson).
const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

const BASE = '/panel/api/scanguard';

export const scanguardApi = {
  status: async (): Promise<Msg<ScanguardStatus>> => {
    const raw = await HttpUtil.get<ScanguardStatus>(`${BASE}/status`, undefined, { silent: true });
    return parseMsg(raw, ScanguardStatusSchema, 'scanguard/status');
  },
  saveConfig: (cfg: ScanguardConfig): Promise<Msg<ScanguardStatus>> =>
    HttpUtil.post<ScanguardStatus>(`${BASE}/config`, cfg, JSON_HEADERS),
  refresh: (): Promise<Msg<ScanguardStatus>> =>
    HttpUtil.post<ScanguardStatus>(`${BASE}/refresh`, {}, JSON_HEADERS),
};
