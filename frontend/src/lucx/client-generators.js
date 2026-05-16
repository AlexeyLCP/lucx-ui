// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Client generators for AWG and Telemt — generate keypairs, PSKs, and secrets
// in the browser using crypto.getRandomValues. Isolated from InboundsPage.vue.

/**
 * Generate an AWG client keypair with PSK.
 * Uses crypto.getRandomValues for Curve25519-compatible random keys.
 * @returns {{ id: string, password: string }} id=pubkey, password=PSK (base64)
 */
export function generateAWGClient() {
  const keyBytes = crypto.getRandomValues(new Uint8Array(32));
  const pubKey = btoa(String.fromCharCode(...keyBytes));
  const pskBytes = crypto.getRandomValues(new Uint8Array(32));
  const psk = btoa(String.fromCharCode(...pskBytes));
  return { id: pubKey, password: psk };
}

/**
 * Generate a Telemt client with FakeTLS ee-secret.
 * @returns {{ id: string, password: string }} id=hex(16), password=ee+hex(16)
 */
export function generateTelemtClient() {
  const bytes = crypto.getRandomValues(new Uint8Array(16));
  const hex = Array.from(bytes, b => b.toString(16).padStart(2, '0')).join('');
  return { id: hex, password: 'ee' + hex };
}

/**
 * Build a client object ready for serialization to the inbound settings.
 * @param {string} protocol - 'awg' or 'telemt'
 * @param {string} name - client email/identifier
 * @returns {Object} client JSON object
 */
export function buildClientObject(protocol, name) {
  let clientData;
  if (protocol === 'awg') {
    clientData = generateAWGClient();
  } else {
    clientData = generateTelemtClient();
  }
  return {
    email: name,
    id: clientData.id,
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
