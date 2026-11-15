// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { fireEvent, screen, waitFor } from '@testing-library/react';
import { beforeEach, describe, expect, it, vi } from 'vitest';

import RouteTester from '@/pages/xray/routing/RouteTester';
import { renderWithProviders } from './test-utils';

const { post } = vi.hoisted(() => ({ post: vi.fn() }));

vi.mock('@/utils', async (importOriginal) => {
  const original = await importOriginal<typeof import('@/utils')>();
  return { ...original, HttpUtil: { post } };
});

vi.mock('@/api/queries/useInboundOptions', () => ({
  useInboundOptions: () => ({ data: [] }),
}));

beforeEach(() => {
  post.mockReset();
  post.mockResolvedValue({ success: true, obj: { matched: true, outboundTag: 'warp' } });
});

describe('route tester', () => {
  it('asks the core about a per-client rule', async () => {
    renderWithProviders(<RouteTester inboundTags={[]} isMobile={false} />);

    fireEvent.change(screen.getByLabelText('Domain or IP'), {
      target: { value: 'example.com' },
    });
    fireEvent.change(screen.getByLabelText('Source IP'), {
      target: { value: '10.200.0.5' },
    });
    fireEvent.change(screen.getByLabelText('User (email)'), {
      target: { value: 'client@example.com' },
    });
    fireEvent.click(screen.getByRole('button', { name: /Test Route/ }));

    await waitFor(() => expect(post).toHaveBeenCalled());
    expect(post).toHaveBeenCalledWith(
      '/panel/api/xray/routeTest',
      expect.objectContaining({
        domain: 'example.com',
        sourceIP: '10.200.0.5',
        email: 'client@example.com',
      }),
    );
  });
});
