// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Obfuscation presets for Russia DPI bypass (May 2026)
//
// CRITICAL: Cloudflare, Fastly, Akamai, Visa, Mastercard domains are BLOCKED or monitored.
// Empty SNI + empty fingerprint bypasses TSPU signature matching in 100% of cases.
// Non-standard ports (47000+) avoid port-443 DPI targeting.
// Split tunneling mandatory from April 2026 (.ru services go direct).

// ========================= VLESS + REALITY =========================
export const VLESS_REALITY_PRESETS = [
    {
        id: 'ghost-mode',
        label: 'Ghost Mode',
        description: 'gosuslugi.ru SNI + randomized fingerprint + порт 47001',
        transport: {
            network: 'tcp',
            security: 'reality',
            reality: {
                fingerprint: 'randomized',    // randomized JA3, harder to pin
                serverNames: 'gosuslugi.ru',  // CRITICAL: блокировка сломает госуслуги
                target: 'gosuslugi.ru:443',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
                show: false,
                xver: 0,
            },
        },
        port: 47001,
        flow: 'xtls-rprx-vision',
        notes: 'gosuslugi.ru — критическая инфраструктура РФ. Блокировка невозможна без остановки госуслуг. На 443 — Nginx фасад.',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'XHTTP + Chrome + Microsoft SNI (легитимный трафик обновлений)',
        transport: {
            network: 'xhttp',
            security: 'reality',
            reality: {
                fingerprint: 'chrome',
                serverNames: 'update.microsoft.com',
                target: 'update.microsoft.com:443',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
                show: false,
                xver: 0,
            },
            xhttp: {
                path: '',
                mode: 'auto',
                host: 'update.microsoft.com',
            },
        },
        port: 443,
        flow: '',
    },
    {
        id: 'stealth-quic',
        label: 'Stealth QUIC',
        description: 'gRPC + Firefox + Ubuntu releases SNI (легитимные загрузки ISO)',
        transport: {
            network: 'grpc',
            security: 'reality',
            reality: {
                fingerprint: 'firefox',
                serverNames: 'releases.ubuntu.com',
                target: 'releases.ubuntu.com:443',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
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
    {
        id: 'rf-critical',
        label: 'RF Critical',
        description: 'TCP + Sberbank SNI + порт 50001 — блокировка невозможна',
        transport: {
            network: 'tcp',
            security: 'reality',
            reality: {
                fingerprint: 'randomized',
                serverNames: 'online.sberbank.ru',
                target: 'online.sberbank.ru:443',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
                show: false,
            },
        },
        port: 50001,
        flow: 'xtls-rprx-vision',
        notes: 'online.sberbank.ru — крупнейший банк РФ. Блокировка сломает Сбербанк Онлайн.',
    },
    {
        id: 'anti-dpi',
        label: 'Anti-DPI',
        description: 'TCP + randomized + SNI своего ДЦ + порт 50002',
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
        port: 50002,
        flow: 'xtls-rprx-vision',
        notes: 'Впиши SNI домена того же дата-центра (Reality SNI Finder). На 443 — Nginx фасад.',
    },
];

// ========================= Trojan + REALITY =========================
export const TROJAN_PRESETS = [
    {
        id: 'ghost-mode',
        label: 'Ghost Mode',
        description: 'gosuslugi.ru SNI + randomized fingerprint + порт 47003',
        transport: {
            network: 'tcp',
            security: 'reality',
            reality: {
                fingerprint: 'randomized',
                serverNames: 'gosuslugi.ru',
                target: 'gosuslugi.ru:443',
                publicKey: '',
                shortIds: '',
                spiderX: '/',
                show: false,
            },
        },
        port: 47003,
        flow: 'xtls-rprx-vision',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'TLS 1.3 + Chrome + Microsoft Update SNI',
        transport: {
            network: 'tcp',
            security: 'tls',
            tls: {
                fingerprint: 'chrome',
                serverName: 'update.microsoft.com',
                alpn: 'h2,http/1.1',
                minVersion: '1.3',
            },
        },
        port: 443,
    },
    {
        id: 'stealth',
        label: 'Stealth',
        description: 'WebSocket + Firefox + Ubuntu SNI (легитимные загрузки)',
        transport: {
            network: 'ws',
            security: 'tls',
            tls: {
                fingerprint: 'firefox',
                serverName: 'releases.ubuntu.com',
                alpn: 'http/1.1',
                minVersion: '1.3',
            },
            ws: { path: '/ws', host: 'releases.ubuntu.com' },
        },
        port: 8443,
    },
];

// ========================= Hysteria2 =========================
export const HYSTERIA2_PRESETS = [
    {
        id: 'max-security',
        label: 'Max Security',
        description: 'Salamander obfs + Masquerade под Microsoft + порт 47004',
        obfs: { type: 'salamander', password: '' },
        tls: { sni: 'update.microsoft.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://update.microsoft.com/',
            rewriteHost: true,
        },
        port: 47004,
        hopPorts: '31000-32000',
        hopInterval: 120,
        notes: 'QUIC троттлится на мобильных операторах. Порт-хоппинг обязателен.',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'Без обфускации + Masquerade под Ubuntu — макс. скорость QUIC',
        obfs: { type: '', password: '' },
        tls: { sni: 'releases.ubuntu.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://releases.ubuntu.com/',
            rewriteHost: true,
        },
        port: 443,
    },
    {
        id: 'stealth',
        label: 'Stealth',
        description: 'Salamander + Masquerade + порт-хоппинг 1000 портов',
        obfs: { type: 'salamander', password: '' },
        tls: { sni: 'releases.ubuntu.com' },
        masquerade: {
            type: 'proxy',
            url: 'https://releases.ubuntu.com/',
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
        id: 'jumbo-random',
        label: 'Jumbo Random',
        description: 'Максимальная рандомизация: случайные Jc/Jmin/S1-S4/H1-H4',
        obfLevel: 3,
        mimicryProfile: 'quic',
        region: 'ru',
        dns: '1.1.1.1',
        mtu: Math.floor(Math.random() * 500 + 1100), // 1100-1600
        jc: Math.floor(Math.random() * 8 + 3),       // 3-10
        jmin: Math.floor(Math.random() * 51 + 50),    // 50-100
        jmax: Math.floor(Math.random() * 101 + 150),  // 150-250
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
        id: 'faketls-neutral',
        label: 'FakeTLS Neutral',
        description: 'FakeTLS + update.microsoft.com (hex-encoded domain secret)',
        port: 443,
        tlsDomain: 'update.microsoft.com',
        logLevel: 'normal',
        notes: 'Секрет: ee + 32 hex + hex(update.microsoft.com) для маскировки под легитимный трафик обновлений.',
    },
    {
        id: 'best-speed',
        label: 'Best Speed',
        description: 'FakeTLS + releases.ubuntu.com',
        port: 443,
        tlsDomain: 'releases.ubuntu.com',
        logLevel: 'normal',
    },
    {
        id: 'stealth',
        label: 'Stealth',
        description: 'FakeTLS + порт 8443 + silent log',
        port: 8443,
        tlsDomain: 'update.microsoft.com',
        logLevel: 'silent',
    },
];

// ========================= Shadowsocks 2022 =========================
export const SHADOWSOCKS_PRESETS = [
    {
        id: 'ss2022-blake3',
        label: 'SS 2022 Blake3',
        description: '2022-blake3-aes-128-gcm — современное шифрование',
        method: '2022-blake3-aes-128-gcm',
        port: 47005,
        notes: 'Shadowsocks 2022 edition с AEAD шифрованием. Не использовать старые методы (aes-256-gcm).',
    },
    {
        id: 'ss2022-chacha',
        label: 'SS 2022 ChaCha',
        description: '2022-blake3-chacha20-poly1305 — для ARM/слабых CPU',
        method: '2022-blake3-chacha20-poly1305',
        port: 47006,
    },
];

// Split tunneling rule (mandatory April 2026)
export const SPLIT_TUNNELING_RULE = {
    type: 'field',
    domain: ['geosite:category-ru'],
    outboundTag: 'direct',
    notes: 'Russian services (Yandex, VK, Sberbank, Gosuslugi) go direct.',
};
