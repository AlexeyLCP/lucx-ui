import { describe, expect, it } from 'vitest';

import { awgPortableIField } from '@/lib/xray/awg-descriptor';

// This table is the twin of portableIFieldCases in
// internal/awg/idescriptor_test.go — keep the two in step, there is no
// generator that does it for us.
//
// The verdicts come from reading both parsers and from probes on a live kernel
// module (v3.1.20260812) recorded in lab/plans/awg-cps-facts.md. Where the two
// engines disagree the portable answer is 'no': this predicate guards what the
// panel hands a client whose engine it cannot know.
const cases: Array<[string, boolean, string]> = [
  ['<b 0x160301><r 64>', true, 'what our own generator emits'],
  ['<b 0xAABB>', true, 'hex digits may be upper case'],
  ['<t>', true, 'timestamp takes no argument'],
  ['<r 0>', true, 'zero is a legal count for both engines'],
  ['<rc 16><rd 8>', true, 'counted tags chain'],

  [' <b 0x41>', true, 'padding around the value never reaches a tag parser'],
  ['<b 0x41>\t', true, 'and the .conf line format strips it either way'],

  ['<c>', false, 'kernel-only; aborts amneziawg-go after it drops every peer'],
  ['<d>', false, 'go-only; the kernel refuses the whole config'],
  ['<ds>', false, 'go-only'],
  ['<dz 4>', false, 'go-only'],

  ['<r -5>', false, 'negative count crashes one engine and ENOMEMs the other'],
  ['<r +5>', false, 'both engines take the plus, but nothing emits it'],
  ['<r abc>', false, 'not a number'],
  ['<r 0x10>', false, 'count is base 10'],
  ['<x 5>', false, 'unknown tag'],
  ['<B 0x16>', false, 'tag names are case sensitive in both engines'],

  ['<b 0x>', false, 'empty literal; both engines refuse it'],
  ['<b 0x123>', false, 'odd number of hex digits'],
  ['<b 0xZZ>', false, 'not hex'],
  ['<b 0X41>', false, 'the 0x prefix is lower case for the kernel'],
  ['<b 41>', false, 'go tolerates a missing 0x, the kernel does not'],
  ['<b>', false, 'literal without an argument'],

  ['<b 0x41', false, 'unclosed; the kernel takes it, go refuses'],
  ['<b  0x41>', false, 'the kernel splits on the first space only'],
  ['<b 0x41 >', false, 'trailing space becomes part of the argument'],
  ['<r 8 >', false, 'same for a count'],
  ['<b 0x41 junk>', false, 'go drops the extra token, the kernel refuses'],

  ['helloworld', false, 'no tags at all; both engines accept it and obfuscate nothing'],
  ['junk<b 0x41>', false, 'text before a tag is silently dropped by both'],
  ['<b 0x41>junk', false, 'and after'],
  ['<b 0x41>mid<r 4>', false, 'and between'],
  ['', false, 'an empty value must never be written as a line'],
  ['   ', false, 'nor a blank one — it refuses the client’s whole config'],
  ['<>', false, 'empty tag'],
  ['<', false, 'a lone bracket'],
  ['>', false, 'the kernel reads this as zero tags'],
];

describe('awgPortableIField', () => {
  it.each(cases)('%j → %s', (input, want, why) => {
    expect(awgPortableIField(input), why).toBe(want);
  });

  it('accepts what the panel generates', () => {
    for (const v of [
      '<b 0x16030101430100013f0303><r 32><b 0x20><r 32>',
      '<b 0x1403030001011703030030><r 48>',
      '<r 103>',
      '<rc 16>',
      '<rd 8>',
      '<t>',
    ]) {
      expect(awgPortableIField(v), `the panel emits ${v} and the predicate refuses it`).toBe(true);
    }
  });

  it('treats undefined as unset, not as a value', () => {
    expect(awgPortableIField(undefined)).toBe(false);
  });
});
