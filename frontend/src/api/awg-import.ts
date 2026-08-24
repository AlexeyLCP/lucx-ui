// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

import { HttpUtil, type Msg } from '@/utils';
import { parseMsg } from '@/utils/zodValidate';
import {
  AwgImportPreviewSchema,
  AwgImportResultSchema,
  type AwgImportPreview,
  type AwgImportResult,
} from '@/schemas/awg-import';

const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };
const BASE = '/panel/api/inbounds/awg/import';

export const awgImportApi = {
  preview: async (): Promise<Msg<AwgImportPreview>> => {
    const raw = await HttpUtil.get<AwgImportPreview>(`${BASE}/preview`, undefined, { silent: true });
    return parseMsg(raw, AwgImportPreviewSchema, 'awg-import/preview');
  },
  dismiss: (): Promise<Msg<null>> => HttpUtil.post<null>(`${BASE}/dismiss`, {}),
  commit: async (ids: string[]): Promise<Msg<AwgImportResult[]>> => {
    const raw = await HttpUtil.post<AwgImportResult[]>(`${BASE}/commit`, { ids }, JSON_HEADERS);
    return parseMsg(raw, z.array(AwgImportResultSchema), 'awg-import/commit');
  },
};
