// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { z } from 'zod';

export const AwgImportPeerSchema = z.object({
  email: z.string(),
  allowedIPs: z.string(),
  publicKey: z.string(),
  hasKey: z.boolean(),
  suspended: z.boolean(),
});

export const AwgImportCandidateSchema = z.object({
  id: z.string(),
  source: z.string(),
  ifname: z.string(),
  confPath: z.string(),
  live: z.boolean(),
  port: z.number(),
  address: z.string(),
  awgVersion: z.string(),
  peerCount: z.number(),
  namedPeers: z.number(),
  keysFound: z.number(),
  handshakes: z.number(),
  suspended: z.number(),
  backend: z.string(),
  dropOnImport: z.boolean(),
  warning: z.string(),
  stopTarget: z.string().optional(),
  peers: z.preprocess((v) => v ?? [], z.array(AwgImportPeerSchema)),
});

export const AwgImportPreviewSchema = z.object({
  dismissed: z.boolean(),
  candidates: z.preprocess((v) => v ?? [], z.array(AwgImportCandidateSchema)),
});

export const AwgImportResultSchema = z.object({
  id: z.string(),
  inboundId: z.number(),
  remark: z.string(),
  clients: z.number(),
  missingKeys: z.number(),
  adopted: z.boolean(),
  stopped: z.boolean().optional(),
  error: z.string().optional(),
});

export type AwgImportCandidate = z.infer<typeof AwgImportCandidateSchema>;
export type AwgImportPreview = z.infer<typeof AwgImportPreviewSchema>;
export type AwgImportResult = z.infer<typeof AwgImportResultSchema>;
