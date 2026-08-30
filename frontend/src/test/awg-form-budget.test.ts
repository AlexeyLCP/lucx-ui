import { describe, expect, it } from 'vitest';

import { AwgInboundSettingsSchema } from '@/schemas/protocols/inbound/awg';

function parse(over: Record<string, unknown>) {
  return AwgInboundSettingsSchema.safeParse(over);
}

function budgetIssues(over: Record<string, unknown>) {
  const res = parse(over);
  if (res.success) return [];
  return res.error.issues.filter((i) => i.message === 'pages.inbounds.form.awgIFieldBudget');
}

// The server now stores an over-budget set and only warns, so the form is the
// last place with an author of the change to refuse — and the only hard stop.
describe('AwgInboundSettingsSchema I-field budget', () => {
  it('accepts a set that lands exactly on the 3492-byte budget', () => {
    expect(parse({ i1: 'x'.repeat(3484) }).success).toBe(true);
  });

  it('refuses the 3604-byte set from the field report', () => {
    const issues = budgetIssues({ i1: 'x'.repeat(3596) });
    expect(issues).toHaveLength(1);
    expect(issues[0].path).toEqual(['i1']);
  });

  it('refuses a set spread across all five fields', () => {
    const spread = Object.fromEntries(
      ['i1', 'i2', 'i3', 'i4', 'i5'].map((k) => [k, 'x'.repeat(720)]),
    );
    expect(budgetIssues(spread)).toHaveLength(1);
  });

  it('charges a header protection key the 36 bytes that drop the budget to 3456', () => {
    const between = 'x'.repeat(3480); // 3488 IBytes: inside 3492, outside 3456
    expect(parse({ i1: between }).success).toBe(true);
    expect(budgetIssues({ i1: between, headerProtectionKey: 'aBcD...base64hpk==' })).toHaveLength(
      1,
    );
  });

  it('does not charge a blank key the 36 bytes a real one costs', () => {
    expect(parse({ i1: 'x'.repeat(3480), headerProtectionKey: '   ' }).success).toBe(true);
  });

  it('measures the trimmed value, exactly as the renderers do', () => {
    expect(parse({ i1: `  ${'x'.repeat(3484)}  ` }).success).toBe(true);
  });

  it('leaves the AWG 1.5 H-range refusal alone', () => {
    const issues = parse({ awgVersion: '1.5', h1: '10-20' });
    expect(issues.success).toBe(false);
    if (issues.success) return;
    expect(issues.error.issues.map((i) => i.message)).toContain('pages.inbounds.form.awgH15Range');
  });
});
