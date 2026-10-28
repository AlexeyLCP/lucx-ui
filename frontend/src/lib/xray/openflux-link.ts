// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

const PREFIX = 'openflux://v1/';

export interface OpenfluxLinkInput {
  secret: string;
  host: string;
  port: number;
  remark?: string;
  yandexUrl?: string;
  mailruUrl?: string;
  cupsUrl?: string;
  vyandexUrl?: string;
  boardsUrl?: string;
  telemostUrl?: string;
  avitoUrl?: string;
}

function storedDeflate(data: Uint8Array): Uint8Array {
  const blocks: number[] = [];
  let offset = 0;
  do {
    const len = Math.min(data.length - offset, 65535);
    const final = offset + len >= data.length;
    const nlen = len ^ 0xffff;
    blocks.push(final ? 1 : 0, len & 0xff, (len >> 8) & 0xff, nlen & 0xff, (nlen >> 8) & 0xff);
    for (let i = 0; i < len; i++) blocks.push(data[offset + i]);
    offset += len;
  } while (offset < data.length);
  return Uint8Array.from(blocks);
}

function b64url(bytes: Uint8Array): string {
  let binary = '';
  for (const b of bytes) binary += String.fromCharCode(b);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
}

export function openfluxContext(input: OpenfluxLinkInput): string {
  return (
    input.yandexUrl ||
    input.mailruUrl ||
    input.cupsUrl ||
    // LUCX-HOOK: carriers shareable since core v0.13.0 (#148), after the
    // original three so existing inbounds keep their context.
    input.vyandexUrl ||
    input.boardsUrl ||
    input.telemostUrl ||
    input.avitoUrl ||
    'http://openflux'
  );
}

export function encodeOpenfluxLink(input: OpenfluxLinkInput): string {
  const secret = input.secret.trim();
  const host = input.host.trim();
  if (secret.length < 16 || !host || input.port < 1) return '';
  const transports: { type: string; priority: number; dial?: string; url?: string }[] = [
    { type: 'direct', priority: 100, dial: `${host}:${input.port}` },
  ];
  // LUCX-HOOK (#148): same priority order as the Go side — descending
  // upstream carrier order under the direct-first scheme.
  if (input.vyandexUrl) transports.push({ type: 'vyandex', priority: 52, url: input.vyandexUrl });
  if (input.yandexUrl) transports.push({ type: 'yandex', priority: 50, url: input.yandexUrl });
  if (input.mailruUrl) transports.push({ type: 'mailru', priority: 40, url: input.mailruUrl });
  if (input.telemostUrl)
    transports.push({ type: 'telemost', priority: 35, url: input.telemostUrl });
  if (input.boardsUrl) transports.push({ type: 'boards', priority: 32, url: input.boardsUrl });
  if (input.avitoUrl) transports.push({ type: 'avito', priority: 31, url: input.avitoUrl });
  if (input.cupsUrl) transports.push({ type: 'cupsonline', priority: 30, url: input.cupsUrl });
  const payload = {
    name: input.remark?.trim() || undefined,
    negotiate: true,
    secret,
    context: openfluxContext(input),
    transports,
  };
  const raw = new TextEncoder().encode(JSON.stringify(payload));
  return PREFIX + b64url(storedDeflate(raw));
}
