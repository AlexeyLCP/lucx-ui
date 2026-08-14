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
  OlcrtcStatusSchema,
  QwdttStatusSchema,
  MieruStatusSchema,
  TrustTunnelStatusSchema,
  type NaiveConfig,
  type NaiveStatus,
  type OlcrtcConfig,
  type OlcrtcStatus,
  type QwdttConfig,
  type QwdttStatus,
  type MieruStatus,
  type TrustTunnelStatus,
} from '@/schemas/tunnel';

export type { NaiveConfig, NaiveStatus, OlcrtcConfig, OlcrtcStatus, QwdttConfig, QwdttStatus, MieruStatus, TrustTunnelStatus };

// JSON_HEADERS is load-bearing on every POST (lucx.69 lesson).
const JSON_HEADERS = { headers: { 'Content-Type': 'application/json' } };

const NAIVE = '/panel/api/tunnel/naive';
const OLCRTC = '/panel/api/tunnel/olcrtc';
const QWDTT = '/panel/api/tunnel/qwdtt';
const MIERU = '/panel/api/tunnel/mieru';
const TRUSTTUNNEL = '/panel/api/tunnel/trusttunnel';

export const tunnelsApi = {
  status: async (): Promise<Msg<NaiveStatus>> => {
    const raw = await HttpUtil.get<NaiveStatus>(`${NAIVE}/status`, undefined, { silent: true });
    return parseMsg(raw, NaiveStatusSchema, 'tunnel/status');
  },
  config: async (): Promise<Msg<NaiveConfig>> => {
    const raw = await HttpUtil.get<NaiveConfig>(`${NAIVE}/config`, undefined, { silent: true });
    return parseMsg(raw, NaiveConfigSchema, 'tunnel/config');
  },
  saveConfig: async (cfg: NaiveConfig): Promise<Msg<NaiveStatus>> => {
    const raw = await HttpUtil.post<NaiveStatus>(`${NAIVE}/config`, cfg, JSON_HEADERS);
    return parseMsg(raw, NaiveStatusSchema, 'tunnel/saveConfig');
  },
  start: (): Promise<Msg<null>> => HttpUtil.post<null>(`${NAIVE}/start`, {}, JSON_HEADERS),
  stop: (): Promise<Msg<null>> => HttpUtil.post<null>(`${NAIVE}/stop`, {}, JSON_HEADERS),
  restart: (): Promise<Msg<null>> => HttpUtil.post<null>(`${NAIVE}/restart`, {}, JSON_HEADERS),
  logs: (lines = 200): Promise<Msg<string[]>> =>
    HttpUtil.get<string[]>(`${NAIVE}/logs?lines=${lines}`),
  preview: async (cfg: NaiveConfig): Promise<Msg<{ caddyfile: string }>> => {
    const raw = await HttpUtil.post<{ caddyfile: string }>(`${NAIVE}/preview`, cfg, JSON_HEADERS);
    return parseMsg(raw, z.object({ caddyfile: z.string() }), 'tunnel/preview');
  },
  validate: async (text: string): Promise<Msg<{ valid: boolean }>> => {
    const raw = await HttpUtil.post<{ valid: boolean }>(`${NAIVE}/validate`, { text }, JSON_HEADERS);
    return parseMsg(raw, z.object({ valid: z.boolean() }), 'tunnel/validate');
  },
  download: (url: string, sha256?: string): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${NAIVE}/download`, { url, sha256 }, JSON_HEADERS),
  upload: (file: File): Promise<Msg<null>> => {
    const fd = new FormData();
    fd.append('file', file);
    return HttpUtil.post<null>(`${NAIVE}/upload`, fd);
  },
  deleteBinary: (): Promise<Msg<null>> => HttpUtil.post<null>(`${NAIVE}/deleteBinary`, {}, JSON_HEADERS),

  olcrtcStatus: async (): Promise<Msg<OlcrtcStatus>> => {
    const raw = await HttpUtil.get<OlcrtcStatus>(`${OLCRTC}/status`, undefined, { silent: true });
    return parseMsg(raw, OlcrtcStatusSchema, 'tunnel/olcrtcStatus');
  },
  olcrtcSaveConfig: async (cfg: OlcrtcConfig): Promise<Msg<OlcrtcStatus>> => {
    const raw = await HttpUtil.post<OlcrtcStatus>(`${OLCRTC}/config`, cfg, JSON_HEADERS);
    return parseMsg(raw, OlcrtcStatusSchema, 'tunnel/olcrtcSaveConfig');
  },
  olcrtcStart: (): Promise<Msg<null>> => HttpUtil.post<null>(`${OLCRTC}/start`, {}, JSON_HEADERS),
  olcrtcStop: (): Promise<Msg<null>> => HttpUtil.post<null>(`${OLCRTC}/stop`, {}, JSON_HEADERS),
  olcrtcRestart: (): Promise<Msg<null>> => HttpUtil.post<null>(`${OLCRTC}/restart`, {}, JSON_HEADERS),
  olcrtcLogs: (lines = 200): Promise<Msg<string[]>> =>
    HttpUtil.get<string[]>(`${OLCRTC}/logs?lines=${lines}`),
  olcrtcPreview: async (cfg: OlcrtcConfig): Promise<Msg<{ yaml: string }>> => {
    const raw = await HttpUtil.post<{ yaml: string }>(`${OLCRTC}/preview`, cfg, JSON_HEADERS);
    return parseMsg(raw, z.object({ yaml: z.string() }), 'tunnel/olcrtcPreview');
  },
  olcrtcDownload: (url: string, sha256?: string): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${OLCRTC}/download`, { url, sha256 }, JSON_HEADERS),
  olcrtcUpload: (file: File): Promise<Msg<null>> => {
    const fd = new FormData();
    fd.append('file', file);
    return HttpUtil.post<null>(`${OLCRTC}/upload`, fd);
  },
  olcrtcDeleteBinary: (): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${OLCRTC}/deleteBinary`, {}, JSON_HEADERS),

  qwdttStatus: async (): Promise<Msg<QwdttStatus>> => {
    const raw = await HttpUtil.get<QwdttStatus>(`${QWDTT}/status`, undefined, { silent: true });
    return parseMsg(raw, QwdttStatusSchema, 'tunnel/qwdttStatus');
  },
  qwdttSaveConfig: async (cfg: QwdttConfig): Promise<Msg<QwdttStatus>> => {
    const raw = await HttpUtil.post<QwdttStatus>(`${QWDTT}/config`, cfg, JSON_HEADERS);
    return parseMsg(raw, QwdttStatusSchema, 'tunnel/qwdttSaveConfig');
  },
  qwdttStart: (): Promise<Msg<null>> => HttpUtil.post<null>(`${QWDTT}/start`, {}, JSON_HEADERS),
  qwdttStop: (): Promise<Msg<null>> => HttpUtil.post<null>(`${QWDTT}/stop`, {}, JSON_HEADERS),
  qwdttRestart: (): Promise<Msg<null>> => HttpUtil.post<null>(`${QWDTT}/restart`, {}, JSON_HEADERS),
  qwdttLogs: (lines = 200): Promise<Msg<string[]>> =>
    HttpUtil.get<string[]>(`${QWDTT}/logs?lines=${lines}`),
  qwdttDownload: (url: string, sha256?: string): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${QWDTT}/download`, { url, sha256 }, JSON_HEADERS),
  qwdttUpload: (file: File): Promise<Msg<null>> => {
    const fd = new FormData();
    fd.append('file', file);
    return HttpUtil.post<null>(`${QWDTT}/upload`, fd);
  },
  qwdttDeleteBinary: (): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${QWDTT}/deleteBinary`, {}, JSON_HEADERS),

  mieruStatus: async (): Promise<Msg<MieruStatus>> => {
    const raw = await HttpUtil.get<MieruStatus>(`${MIERU}/status`, undefined, { silent: true });
    return parseMsg(raw, MieruStatusSchema, 'tunnel/mieruStatus');
  },
  mieruLogs: (lines = 200): Promise<Msg<string[]>> =>
    HttpUtil.get<string[]>(`${MIERU}/logs?lines=${lines}`),
  mieruDownload: (url: string, sha256?: string): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${MIERU}/download`, { url, sha256 }, JSON_HEADERS),
  mieruUpload: (file: File): Promise<Msg<null>> => {
    const fd = new FormData();
    fd.append('file', file);
    return HttpUtil.post<null>(`${MIERU}/upload`, fd);
  },
  mieruDeleteBinary: (): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${MIERU}/deleteBinary`, {}, JSON_HEADERS),

  trustTunnelStatus: async (): Promise<Msg<TrustTunnelStatus>> => {
    const raw = await HttpUtil.get<TrustTunnelStatus>(`${TRUSTTUNNEL}/status`, undefined, { silent: true });
    return parseMsg(raw, TrustTunnelStatusSchema, 'tunnel/trustTunnelStatus');
  },
  trustTunnelLogs: (lines = 200): Promise<Msg<string[]>> =>
    HttpUtil.get<string[]>(`${TRUSTTUNNEL}/logs?lines=${lines}`),
  trustTunnelDownload: (url: string, sha256?: string): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${TRUSTTUNNEL}/download`, { url, sha256 }, JSON_HEADERS),
  trustTunnelUpload: (file: File): Promise<Msg<null>> => {
    const fd = new FormData();
    fd.append('file', file);
    return HttpUtil.post<null>(`${TRUSTTUNNEL}/upload`, fd);
  },
  trustTunnelDeleteBinary: (): Promise<Msg<null>> =>
    HttpUtil.post<null>(`${TRUSTTUNNEL}/deleteBinary`, {}, JSON_HEADERS),
};
