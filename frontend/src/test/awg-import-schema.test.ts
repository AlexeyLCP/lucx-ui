// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { describe, it, expect } from 'vitest';
import { AwgImportPreviewSchema, AwgImportResultSchema } from '@/schemas/awg-import';

describe('AwgImportPreviewSchema', () => {
  it('treats null candidates as empty', () => {
    const got = AwgImportPreviewSchema.parse({ dismissed: false, candidates: null });
    expect(got.candidates).toEqual([]);
  });

  it('treats missing candidates as empty', () => {
    const got = AwgImportPreviewSchema.parse({ dismissed: true });
    expect(got.candidates).toEqual([]);
  });
});

describe('AwgImportResultSchema', () => {
  // parseMsg keeps only what the schema declares, so an undeclared field is
  // dropped between the panel and the UI rather than failing loudly.
  it('keeps the saved-with-warning note beside a real failure', () => {
    const got = AwgImportResultSchema.parse({
      id: 'amnezia:awg0',
      inboundId: 12,
      remark: 'awg0',
      clients: 3,
      missingKeys: 0,
      adopted: true,
      stopped: false,
      error: 'saved, stop old source failed: boom',
      warning: 'saved, I-fields will not be applied',
    });
    expect(got.warning).toBe('saved, I-fields will not be applied');
    expect(got.error).toBe('saved, stop old source failed: boom');
  });
});
