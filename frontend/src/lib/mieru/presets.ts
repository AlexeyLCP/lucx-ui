// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import type {
  MieruHandshakeMode,
  MieruMultiplexing,
  MieruTrafficPattern,
} from '@/schemas/protocols/inbound/mieru';

export type MieruPresetName = 'off' | 'lite' | 'standard' | 'stealth';

export interface MieruPresetValues {
  multiplexing?: MieruMultiplexing;
  handshakeMode?: MieruHandshakeMode;
  trafficPattern?: MieruTrafficPattern;
}

function randomSeed(): number {
  return Math.floor(Math.random() * 2147483646) + 1;
}

export function mieruPreset(name: MieruPresetName): MieruPresetValues {
  switch (name) {
    case 'lite':
      return {
        multiplexing: 'MULTIPLEXING_LOW',
        handshakeMode: 'HANDSHAKE_STANDARD',
        trafficPattern: {
          seed: randomSeed(),
          padding: { maxMiddlePaddingLen: 16, maxEndPaddingLen: 16 },
        },
      };
    case 'standard':
      return {
        multiplexing: 'MULTIPLEXING_MIDDLE',
        handshakeMode: 'HANDSHAKE_STANDARD',
        trafficPattern: {
          seed: randomSeed(),
          tcpFragment: { enable: true, maxSleepMs: 10 },
          padding: { maxMiddlePaddingLen: 64, maxEndPaddingLen: 32 },
        },
      };
    case 'stealth':
      return {
        multiplexing: 'MULTIPLEXING_HIGH',
        handshakeMode: 'HANDSHAKE_NO_WAIT',
        trafficPattern: {
          seed: randomSeed(),
          tcpFragment: { enable: true, maxSleepMs: 20 },
          nonce: { type: 'NONCE_TYPE_PRINTABLE', applyToAllUDPPacket: true },
          padding: { maxMiddlePaddingLen: 128, maxEndPaddingLen: 64 },
          lowEntropy: {
            mode: 'LOW_ENTROPY_MODE_48',
            maskRotation: 'LOW_ENTROPY_MASK_ROTATE_RIGHT_4',
          },
        },
      };
    case 'off':
      return {};
  }
}
