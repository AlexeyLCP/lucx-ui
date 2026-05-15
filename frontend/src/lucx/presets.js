// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Obfuscation presets optimized for Russia DPI blocking (May 2026)
// Based on community research: TSPU targets port 443, Apple/Microsoft SNI are burned,
// empty SNI + empty fingerprint bypasses 100% of signature-based blocking.

export const VLESS_REALITY_PRESETS = [
    {
        id: 'max-security',
        label: 'Max Security',
        description: 'Пустой SNI + пустой fingerprint + порт 47001 — обход сигнатур ТСПУ',
        transport: {
            network: 'tcp',
            security: 'reality',
            reality: {
                fingerprint: '',
                serverNames: '',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
                show: false,
                xver: 0,
            },
        },
        port: 47001,
        flow: 'xtls-rprx-vision',
        notes: 'На порту 443 должен быть Nginx/Caddy фасад с реальным сайтом для защиты от Active Probing',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'XHTTP + Chrome + Cloudflare CDN SNI + xmux — высокая скорость',
        transport: {
            network: 'xhttp',
            security: 'reality',
            reality: {
                fingerprint: 'chrome',
                serverNames: 'cdnjs.cloudflare.com',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
                show: false,
                xver: 0,
            },
            xhttp: {
                path: '',
                mode: 'auto',
                host: 'cdnjs.cloudflare.com',
            },
        },
        port: 443,
        flow: '',
    },
    {
        id: 'stealth-cdn',
        label: 'Stealth CDN',
        description: 'WebSocket + Firefox + Cloudflare SNI — за CDN не видно реальный IP',
        transport: {
            network: 'ws',
            security: 'reality',
            reality: {
                fingerprint: 'firefox',
                serverNames: 'ajax.cloudflare.com',
                publicKey: '',
                shortIds: '',
                spiderX: '/api/v1/',
                show: false,
            },
            ws: {
                path: '/ws',
                host: 'ajax.cloudflare.com',
            },
        },
        port: 443,
        flow: 'xtls-rprx-vision',
    },
    {
        id: 'anti-dpi',
        label: 'Anti-DPI',
        description: 'TCP + randomized + SNI того же ДЦ что VPS + порт 47002',
        transport: {
            network: 'tcp',
            security: 'reality',
            reality: {
                fingerprint: 'randomized',
                serverNames: '',
                publicKey: '',
                shortIds: '',
                spiderX: '/download',
                show: false,
            },
        },
        port: 47002,
        flow: 'xtls-rprx-vision',
        notes: 'SNI должен быть доменом, размещённым в том же дата-центре что и VPS. На 443 — Nginx фасад.',
    },
];

export const HYSTERIA2_PRESETS = [
    {
        id: 'max-security',
        label: 'Max Security',
        description: 'Salamander obfs + Cloudflare SNI',
        obfs: { type: 'salamander', password: '' },
        tls: { sni: 'www.cloudflare.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://www.cloudflare.com/',
            rewriteHost: true,
        },
        port: 443,
        notes: 'QUIC может троттлиться на мобильных операторах. Включите порт-хоппинг.',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'Без обфускации + Cloudflare CDN SNI — макс. скорость QUIC',
        obfs: { type: '', password: '' },
        tls: { sni: 'cdnjs.cloudflare.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://cdnjs.cloudflare.com/',
            rewriteHost: true,
        },
        port: 443,
    },
    {
        id: 'stealth',
        label: 'Stealth',
        description: 'Salamander + Bing SNI + порт-хоппинг',
        obfs: { type: 'salamander', password: '' },
        tls: { sni: 'www.bing.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://www.bing.com/',
            rewriteHost: true,
        },
        port: 443,
    },
];
