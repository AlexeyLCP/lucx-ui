// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

/**
 * Generate 32 cryptographically random bytes → base64 string.
 * Produces plain strings, never CryptoKey objects — safe for JSON serialization.
 */
function genBase64(len = 32) {
  const bytes = crypto.getRandomValues(new Uint8Array(len));
  let binary = '';
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}

/**
 * Generate AWG client keypair + PSK. All values are plain base64 strings.
 * Client generates real X25519 keypair on their own machine via `awg genkey`.
 * What we provide: random base64 placeholder keys that pass format validation.
 */
export function generateAWGClient() {
  return {
    id: genBase64(32),        // public key (base64, 32 random bytes)
    privateKey: genBase64(32), // private key (base64, 32 random bytes)
    password: genBase64(32),   // PSK (base64, 32 random bytes)
  };
}

/**
 * Generate Telemt FakeTLS ee-secret.
 */
export function generateTelemtClient() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
  return { id: hex, password: 'ee' + hex };
}

/**
 * Build client object for serialization to inbound settings.
 */
export function buildClientObject(protocol, name) {
  const clientData = protocol === 'awg' ? generateAWGClient() : generateTelemtClient();
  return {
    email: name,
    id: clientData.id,
    privateKey: clientData.privateKey || '',
    password: clientData.password,
    enable: true,
    flow: '',
    limitIP: 0,
    totalGB: 0,
    expiryTime: 0,
    tgId: '',
    subId: '',
    comment: '',
  };
}
