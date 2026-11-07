// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { HttpUtil, Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import { CleanIpResultsSchema, type CleanIpParams, type CleanIpResult } from '@/schemas/cleanip';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

const BASE = '/panel/api/cleanip';

export const cleanIpApi = {
  scan: async (params: CleanIpParams): Promise<Msg<CleanIpResult[]>> => {
    const raw = await HttpUtil.post<CleanIpResult[]>(`${BASE}/scan`, params, JSON_HEADERS);
    return parseMsg(raw, CleanIpResultsSchema, 'cleanip/scan');
  },
};
