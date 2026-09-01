import { describe, expect, it } from 'vitest';

import { DBInbound } from '@/models/dbinbound';
import {
  awgIFieldSetFrom,
  awgIFieldGrammarRefused,
  awgIFieldSetRefused,
  awgSavedIFieldSet,
} from '@/pages/inbounds/form/awgIFieldBudget';

// 3596 chars = 3604 IBytes against a 3492 budget — the set from the field report.
const OVERSIZE = 'x'.repeat(3596);
// 3480 chars = 3488 IBytes: inside 3492, outside the 3456 an HPK leaves.
const BETWEEN = 'x'.repeat(3480);
const HPK = 'MCPfRGcDGotJ6TcnIdDqsemj2cMIiGHnPUHM5ivXN18=';

/*
 * The server stores an over-budget I-set and only warns, so the form is the
 * last refusal — but the rule is "refuse where there is an author of the
 * change", and only the form knows both the new set and the stored one.
 */
describe('awgIFieldSetRefused', () => {
  it('lets an unchanged over-budget set through, so the rest of the form stays editable', () => {
    const stored = { i1: OVERSIZE };
    expect(awgIFieldSetRefused({ i1: OVERSIZE }, stored)).toBe(false);
  });

  it('refuses an over-budget set the operator changed', () => {
    expect(awgIFieldSetRefused({ i1: 'y'.repeat(3596) }, { i1: OVERSIZE })).toBe(true);
  });

  it('refuses an over-budget set on a new inbound, which has nothing stored', () => {
    expect(awgIFieldSetRefused({ i1: OVERSIZE }, null)).toBe(true);
  });

  it('refuses an untouched set once a header protection key drops the budget to 3456', () => {
    const stored = { i1: BETWEEN, headerProtectionKey: '' };
    expect(awgIFieldSetRefused({ i1: BETWEEN, headerProtectionKey: '' }, stored)).toBe(false);
    expect(awgIFieldSetRefused({ i1: BETWEEN, headerProtectionKey: HPK }, stored)).toBe(true);
  });

  // The budget is a property of the RESULT, not of the difference: dropping the
  // key raises the ceiling back to 3492, so a set that now fits is not compared.
  it('saves a grandfathered set once dropping the key brings it inside the budget', () => {
    const stored = { i1: BETWEEN, headerProtectionKey: HPK };
    expect(awgIFieldSetRefused({ i1: BETWEEN, headerProtectionKey: '' }, stored)).toBe(false);
    expect(awgIFieldSetRefused({ i1: 'y'.repeat(3480), headerProtectionKey: '' }, stored)).toBe(
      false,
    );
  });

  it('still refuses when dropping the key leaves the set over the 3492 ceiling', () => {
    const stored = { i1: OVERSIZE, headerProtectionKey: HPK };
    expect(awgIFieldSetRefused({ i1: OVERSIZE, headerProtectionKey: '' }, stored)).toBe(true);
  });

  it('never refuses a set inside the budget, stored or not', () => {
    const small = { i1: 'x'.repeat(100) };
    expect(awgIFieldSetRefused(small, null)).toBe(false);
    expect(awgIFieldSetRefused(small, { i1: OVERSIZE })).toBe(false);
  });

  it('measures and compares trimmed, so a cosmetic space edit is not a new set', () => {
    expect(awgIFieldSetRefused({ i1: `  ${OVERSIZE}  ` }, { i1: OVERSIZE })).toBe(false);
  });

  it('counts all five fields, not just i1', () => {
    const spread = Object.fromEntries(
      ['i1', 'i2', 'i3', 'i4', 'i5'].map((k) => [k, 'x'.repeat(720)]),
    );
    expect(awgIFieldSetRefused(spread, null)).toBe(true);
    expect(awgIFieldSetRefused(spread, spread)).toBe(false);
    expect(awgIFieldSetRefused({ ...spread, i3: 'z'.repeat(720) }, spread)).toBe(true);
  });
});

describe('awgSavedIFieldSet', () => {
  const awgInbound = (settings: Record<string, unknown>) =>
    new DBInbound({ id: 1, protocol: 'awg', settings });

  it('reads the stored set an edit is grandfathered against', () => {
    expect(awgSavedIFieldSet(awgInbound({ i1: OVERSIZE }))?.i1).toBe(OVERSIZE);
  });

  it('reads settings that arrived as a JSON string', () => {
    expect(awgSavedIFieldSet(awgInbound(JSON.stringify({ i1: OVERSIZE }) as never))?.i1).toBe(
      OVERSIZE,
    );
  });

  it('has nothing to grandfather on add or on a switch from another protocol', () => {
    expect(awgSavedIFieldSet(null)).toBeNull();
    expect(awgSavedIFieldSet(new DBInbound({ id: 1, protocol: 'vless', settings: {} }))).toBeNull();
  });
});

describe('awgIFieldSetFrom', () => {
  it('keeps only the six strings the budget depends on', () => {
    expect(awgIFieldSetFrom({ i1: 'a', i2: 2, headerProtectionKey: HPK, jc: 4 })).toEqual({
      i1: 'a',
      i2: '',
      i3: '',
      i4: '',
      i5: '',
      headerProtectionKey: HPK,
    });
  });
});

// The stored value may be someone's working config on the engine it was written
// for, so the form refuses only what this operator just typed.
describe('awgIFieldGrammarRefused', () => {
  it('spares a stored value the operator did not touch', () => {
    const stored = { i1: '<b 0x01>', i3: '<c>' };
    expect(awgIFieldGrammarRefused({ i1: '<t>', i3: '<c>' }, stored)).toEqual([]);
  });

  it('refuses a value typed now', () => {
    expect(awgIFieldGrammarRefused({ i1: '<c>' }, null)).toEqual(['I1']);
  });

  it('refuses a stored value the operator edited into another bad one', () => {
    expect(awgIFieldGrammarRefused({ i3: '<d>' }, { i3: '<c>' })).toEqual(['I3']);
  });

  it('allows fixing one field while another stays bad', () => {
    const stored = { i1: '<c>', i3: '<c>' };
    expect(awgIFieldGrammarRefused({ i1: '<t>', i3: '<c>' }, stored)).toEqual([]);
  });

  it('names every field it refuses, in order', () => {
    expect(awgIFieldGrammarRefused({ i2: '<c>', i5: 'helloworld' }, null)).toEqual(['I2', 'I5']);
  });

  it('does not refuse a blank field', () => {
    expect(awgIFieldGrammarRefused({ i1: '   ', i2: '' }, null)).toEqual([]);
  });
});
