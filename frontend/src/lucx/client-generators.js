// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

/**
 * Generate an AWG client keypair using Web Crypto API (X25519).
 * Returns both private key (for client config) and public key (for server storage).
 * @returns {Promise<{ id: string, privateKey: string, password: string }>}
 */
export async function generateAWGClient() {
  // Generate real X25519 keypair
  const keyPair = await crypto.subtle.generateKey(
    { name: 'X25519' },
    true,   // extractable
    ['deriveBits']
  );

  // Export public key as raw bytes → base64
  const pubRaw = await crypto.subtle.exportKey('raw', keyPair.publicKey);
  const pubKey = btoa(String.fromCharCode(...new Uint8Array(pubRaw)));

  // Export private key as raw bytes → base64
  const privRaw = await crypto.subtle.exportKey('raw', keyPair.privateKey);
  const privateKey = btoa(String.fromCharCode(...new Uint8Array(privRaw)));

  // Generate PSK
  const pskBytes = crypto.getRandomValues(new Uint8Array(32));
  const psk = btoa(String.fromCharCode(...pskBytes));

  return { id: pubKey, privateKey, password: psk };
}

/**
 * Generate a Telemt client with FakeTLS ee-secret.
 * @returns {{ id: string, password: string }}
 */
export function generateTelemtClient() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
  return { id: hex, password: 'ee' + hex };
}

/**
 * Build a client object ready for serialization to the inbound settings.
 * The privateKey field is for one-time config download only — never persisted.
 * @param {string} protocol - 'awg' or 'telemt'
 * @param {string} name - client email/identifier
 * @returns {Promise<Object>}
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
    id: clientData.id,             // public key → stored on server
    privateKey: clientData.privateKey, // private key → config only, never persisted
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
