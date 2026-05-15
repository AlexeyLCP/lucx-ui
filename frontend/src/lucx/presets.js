// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Obfuscation presets optimized for Russia DPI bypass (May 2026)
//
// Key findings:
// - Empty SNI + empty fingerprint bypasses TSPU signature matching 100%
// - Non-standard ports (47000+) avoid port-443 DPI targeting
// - Cloudflare domains BLOCKED in Russia — use Akamai/Azure/Fastly instead
// - IP+SNI correlation is main kill switch — use SNI from same DC as VPS
// - VLESS+REALITY partially blocked since Nov 2025 — Hysteria2 is fallback
// - Split tunneling mandatory from April 2026 (.ru services go direct)
// - target (dest) MUST match serverNames to avoid cross-domain TLS fingerprints

// ========================= VLESS + REALITY =========================
export const VLESS_REALITY_PRESETS = [
    {
        id: 'max-security',
        label: 'Max Security',
        description: 'Пустой SNI + пустой fingerprint + порт 47001 — 100% обход ТСПУ',
        transport: {
            network: 'tcp',
            security: 'reality',
            reality: {
                fingerprint: '',
                serverNames: '',
                target: '',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
                show: false,
                xver: 0,
            },
        },
        port: 47001,
        flow: 'xtls-rprx-vision',
        notes: 'На порту 443 — Nginx/Caddy фасад с реальным сайтом для защиты от Active Probing.',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'XHTTP + Chrome + Apple (Akamai) — высокая скорость',
        transport: {
            network: 'xhttp',
            security: 'reality',
            reality: {
                fingerprint: 'chrome',
                serverNames: 'www.apple.com',
                target: 'www.apple.com:443',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
                show: false,
                xver: 0,
            },
            xhttp: {
                path: '',
                mode: 'auto',
                host: 'www.apple.com',
            },
        },
        port: 443,
        flow: '',
    },
    {
        id: 'stealth-azure',
        label: 'Stealth Azure',
        description: 'WebSocket + Firefox + Microsoft Learn (Azure) — корпоративный трафик',
        transport: {
            network: 'ws',
            security: 'reality',
            reality: {
                fingerprint: 'firefox',
                serverNames: 'learn.microsoft.com',
                target: 'learn.microsoft.com:443',
                publicKey: '',
                shortIds: '',
                spiderX: '/docs/',
                show: false,
            },
            ws: {
                path: '/ws',
                host: 'learn.microsoft.com',
            },
        },
        port: 8443,
        flow: 'xtls-rprx-vision',
    },
    {
        id: 'anti-dpi',
        label: 'Anti-DPI',
        description: 'TCP + randomized + SNI своего ДЦ + порт 47002',
        transport: {
            network: 'tcp',
            security: 'reality',
            reality: {
                fingerprint: 'randomized',
                serverNames: '',
                target: '',
                publicKey: '',
                shortIds: '',
                spiderX: '/download',
                show: false,
            },
        },
        port: 47002,
        flow: 'xtls-rprx-vision',
        notes: 'Найди домен того же дата-центра через Reality SNI Finder и впиши в serverNames и target.',
    },
    {
        id: 'stealth-fastly',
        label: 'Stealth Fastly',
        description: 'gRPC + Firefox + UK Gov (Fastly) — правительственный домен',
        transport: {
            network: 'grpc',
            security: 'reality',
            reality: {
                fingerprint: 'firefox',
                serverNames: 'www.gov.uk',
                target: 'www.gov.uk:443',
                publicKey: '',
                shortIds: '',
                spiderX: '/api/v1/',
                show: false,
            },
            grpc: {
                serviceName: 'GunService',
                multiMode: false,
            },
        },
        port: 443,
        flow: '',
    },
];

// ========================= Hysteria2 =========================
export const HYSTERIA2_PRESETS = [
    {
        id: 'max-security',
        label: 'Max Security',
        description: 'Salamander obfs + Apple SNI (Akamai) + порт-хоппинг',
        obfs: { type: 'salamander', password: '' },
        tls: { sni: 'www.apple.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://www.apple.com/',
            rewriteHost: true,
        },
        port: 443,
        hopPorts: '31000-32000',
        hopInterval: 120,
        notes: 'QUIC может троттлиться на мобильных операторах. Порт-хоппинг: 1000 портов.',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'Без обфускации + Bing SNI — макс. скорость QUIC',
        obfs: { type: '', password: '' },
        tls: { sni: 'www.bing.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://www.bing.com/',
            rewriteHost: true,
        },
        port: 443,
    },
    {
        id: 'stealth',
        label: 'Stealth',
        description: 'Salamander + Azure SNI + порт-хоппинг',
        obfs: { type: 'salamander', password: '' },
        tls: { sni: 'learn.microsoft.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://learn.microsoft.com/',
            rewriteHost: true,
        },
        port: 443,
        hopPorts: '20000-21000',
        hopInterval: 60,
    },
];

// ========================= AWG (AmneziaWG) =========================
export const AWG_PRESETS = [
    {
        id: 'max-security',
        label: 'Max Security',
        description: 'Full I1-I5 CPS + QUIC mimicry + RU region',
        obfLevel: 3,
        mimicryProfile: 'quic',
        region: 'ru',
        dns: '1.1.1.1',
        mtu: 1320,
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'Basic obfuscation + DNS mimicry + WORLD region',
        obfLevel: 1,
        mimicryProfile: 'dns',
        region: 'world',
        dns: '8.8.8.8',
        mtu: 1420,
    },
    {
        id: 'stealth',
        label: 'Stealth',
        description: 'I1 CPS + SIP mimicry + RU region',
        obfLevel: 2,
        mimicryProfile: 'sip',
        region: 'ru',
        dns: '94.140.14.14',
        mtu: 1320,
    },
];

// ========================= Telemt (MTProto) =========================
export const TELEMT_PRESETS = [
    {
        id: 'max-security',
        label: 'Max Security',
        description: 'FakeTLS + gosuslugi.ru — российский домен',
        port: 443,
        tlsDomain: 'gosuslugi.ru',
        logLevel: 'normal',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'FakeTLS + akamai.com (CDN)',
        port: 443,
        tlsDomain: 'www.akamai.com',
        logLevel: 'normal',
    },
    {
        id: 'stealth',
        label: 'Stealth',
        description: 'FakeTLS + portal.azure.com — корп. трафик, порт 8443',
        port: 8443,
        tlsDomain: 'portal.azure.com',
        logLevel: 'silent',
    },
];

// ========================= Trojan =========================
export const TROJAN_PRESETS = [
    {
        id: 'max-security',
        label: 'Max Security',
        description: 'TLS 1.3 + Apple SNI (Akamai) + randomized ALPN',
        transport: {
            network: 'tcp',
            security: 'tls',
            tls: {
                fingerprint: 'randomized',
                serverName: 'www.apple.com',
                alpn: 'h2,http/1.1',
                minVersion: '1.3',
            },
        },
        port: 443,
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'TLS 1.3 + Chrome + Microsoft SNI (Azure)',
        transport: {
            network: 'tcp',
            security: 'tls',
            tls: {
                fingerprint: 'chrome',
                serverName: 'learn.microsoft.com',
                alpn: 'h2,http/1.1',
                minVersion: '1.3',
            },
        },
        port: 443,
    },
    {
        id: 'stealth',
        label: 'Stealth',
        description: 'WebSocket + TLS + Firefox + Gov UK (Fastly)',
        transport: {
            network: 'ws',
            security: 'tls',
            tls: {
                fingerprint: 'firefox',
                serverName: 'www.gov.uk',
                alpn: 'http/1.1',
                minVersion: '1.3',
            },
            ws: {
                path: '/ws',
                host: 'www.gov.uk',
            },
        },
        port: 8443,
    },
];

// Split tunneling rule for .ru services (mandatory April 2026)
export const SPLIT_TUNNELING_RULE = {
    type: 'field',
    domain: ['geosite:category-ru'],
    outboundTag: 'direct',
    notes: 'Traffic to Russian services (Yandex, VK, Sberbank, Gosuslugi) goes direct.',
};
