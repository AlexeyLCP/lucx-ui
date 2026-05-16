// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

/**
 * Generate 32 random bytes as base64 — fallback for environments
 * where Web Crypto X25519 is unavailable.
 */
function genRandomBase64(len = 32) {
  const bytes = crypto.getRandomValues(new Uint8Array(len));
  return btoa(String.fromCharCode(...bytes));
}

/**
 * Generate an AWG client keypair.
 * Tries Web Crypto X25519 first; falls back to random bytes.
 * @returns {Promise<{ id: string, privateKey: string, password: string }>}
 */
export async function generateAWGClient() {
  let pubKey, privateKey;
  try {
    const keyPair = await crypto.subtle.generateKey(
      { name: 'X25519' }, true, ['deriveBits']
    );
    const pubRaw = await crypto.subtle.exportKey('raw', keyPair.publicKey);
    pubKey = btoa(String.fromCharCode(...new Uint8Array(pubRaw)));
    const privRaw = await crypto.subtle.exportKey('raw', keyPair.privateKey);
    privateKey = btoa(String.fromCharCode(...new Uint8Array(privRaw)));
  } catch (_e) {
    // X25519 not available — use random bytes (client generates real key)
    pubKey = genRandomBase64(32);
    privateKey = genRandomBase64(32);
  }
  const psk = genRandomBase64(32);
  return { id: pubKey, privateKey, password: psk };
}

/**
 * Generate a Telemt client with FakeTLS ee-secret.
 */
export function generateTelemtClient() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
  return { id: hex, password: 'ee' + hex };
}

/**
 * Build a client object ready for serialization.
 * privateKey is for one-time config download — never persisted on server.
 */
export async function buildClientObject(protocol, name) {
  let clientData;
  if (protocol === 'awg') {
    clientData = await generateAWGClient();
  } else {
    clientData = generateTelemtClient();
  }
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
