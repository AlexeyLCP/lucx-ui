import { describe, it, expect } from 'vitest';
import { AwgImportPreviewSchema } from '@/schemas/awg-import';

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
