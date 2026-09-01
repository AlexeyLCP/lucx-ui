// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

import { awgIBytes, awgWorstCaseIBytesBudget } from '@/lib/xray/awg-budget';
import { awgPortableIField } from '@/lib/xray/awg-descriptor';
import { coerceInboundJsonField, DBInbound } from '@/models/dbinbound';
import { Protocols } from '@/schemas/primitives';

export interface AwgIFieldSet {
  i1?: string;
  i2?: string;
  i3?: string;
  i4?: string;
  i5?: string;
  headerProtectionKey?: string;
}

const BUDGET_FIELDS = ['i1', 'i2', 'i3', 'i4', 'i5', 'headerProtectionKey'] as const;

// awgIFieldSetFrom narrows any settings blob to the six strings the budget
// depends on, so a stored set and a form value are compared on equal terms.
export function awgIFieldSetFrom(settings: unknown): AwgIFieldSet {
  const raw = (settings ?? {}) as Record<string, unknown>;
  const out: AwgIFieldSet = {};
  for (const key of BUDGET_FIELDS) {
    out[key] = typeof raw[key] === 'string' ? (raw[key] as string) : '';
  }
  return out;
}

// awgSavedIFieldSet is the set an edit is grandfathered against: null on add and
// on a switch from another protocol, neither of which has an AWG set to keep.
export function awgSavedIFieldSet(dbInbound: DBInbound | null): AwgIFieldSet | null {
  if (!dbInbound || dbInbound.protocol !== Protocols.AWG) return null;
  return awgIFieldSetFrom(coerceInboundJsonField(dbInbound.settings));
}

// The threshold itself moves with the key (3492 -> 3456), so "unchanged" has to
// cover it; compared trimmed, because trimmed is what awgIBytes measures.
function sameIFieldSet(a: AwgIFieldSet, b: AwgIFieldSet): boolean {
  return BUDGET_FIELDS.every((k) => (a[k] ?? '').trim() === (b[k] ?? '').trim());
}

// awgIFieldSetRefused: the server stores an over-budget set and only warns, so
// the form refuses — but only a set whose author is the operator editing now.
export function awgIFieldSetRefused(next: AwgIFieldSet, saved: AwgIFieldSet | null): boolean {
  const budget = awgWorstCaseIBytesBudget((next.headerProtectionKey ?? '').trim() !== '');
  if (awgIBytes(next.i1, next.i2, next.i3, next.i4, next.i5) <= budget) return false;
  return saved === null || !sameIFieldSet(next, saved);
}

const I_FIELDS = ['i1', 'i2', 'i3', 'i4', 'i5'] as const;

// awgIFieldGrammarRefused names the fields this operator just typed that no
// client could use. Per field, unlike the budget: grammar is a property of the
// value. And only what changed — a stored descriptor the panel will not export
// may still be someone's working config on the engine it was written for, and
// refusing to save around it would strand the whole inbound.
export function awgIFieldGrammarRefused(next: AwgIFieldSet, saved: AwgIFieldSet | null): string[] {
  return I_FIELDS.filter((key) => {
    const v = (next[key] ?? '').trim();
    if (v === '' || awgPortableIField(v)) return false;
    return saved === null || (saved[key] ?? '').trim() !== v;
  }).map((key) => key.toUpperCase());
}
