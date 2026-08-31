// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import "testing"

// portableIFieldCases is shared with the TypeScript twin in
// frontend/src/lib/xray/awg-descriptor.ts — keep the two tables in step, there
// is no generator that does it for us.
//
// The verdicts come from reading both parsers and from probes on a live kernel
// module (v3.1.20260812) recorded in lab/plans/awg-cps-facts.md. Where the two
// engines disagree the portable answer is "no": this predicate guards what the
// panel hands a client whose engine it cannot know.
var portableIFieldCases = []struct {
	in   string
	want bool
	why  string
}{
	{"<b 0x160301><r 64>", true, "what our own generator emits"},
	{"<b 0xAABB>", true, "hex digits may be upper case"},
	{"<t>", true, "timestamp takes no argument"},
	{"<r 0>", true, "zero is a legal count for both engines"},
	{"<rc 16><rd 8>", true, "counted tags chain"},

	{"<c>", false, "kernel-only; aborts amneziawg-go after it drops every peer"},
	{"<d>", false, "go-only; the kernel refuses the whole config"},
	{"<ds>", false, "go-only"},
	{"<dz 4>", false, "go-only"},

	{"<r -5>", false, "negative count crashes one engine and ENOMEMs the other"},
	{"<r +5>", false, "both engines take the plus, but the portable form has no sign"},
	{"<r abc>", false, "not a number"},
	{"<r 0x10>", false, "count is base 10"},
	{"<x 5>", false, "unknown tag"},
	{"<B 0x16>", false, "tag names are case sensitive in both engines"},

	{"<b 0x>", false, "empty literal; both engines refuse it"},
	{"<b 0x123>", false, "odd number of hex digits"},
	{"<b 0xZZ>", false, "not hex"},
	{"<b 0X41>", false, "the 0x prefix is lower case for the kernel"},
	{"<b 41>", false, "go tolerates a missing 0x, the kernel does not"},
	{"<b>", false, "literal without an argument"},

	{"<b 0x41", false, "unclosed; the kernel takes it, go refuses"},
	{"<b  0x41>", false, "the kernel splits on the first space only"},
	{"<b 0x41 >", false, "trailing space becomes part of the argument"},
	{"<r 8 >", false, "same for a count"},
	{"<b 0x41 junk>", false, "go drops the extra token, the kernel refuses"},

	{"helloworld", false, "no tags at all; both engines accept it and obfuscate nothing"},
	{"junk<b 0x41>", false, "text before a tag is silently dropped by both"},
	{"<b 0x41>junk", false, "and after"},
	{"<b 0x41>mid<r 4>", false, "and between"},
	{"", false, "an empty value must never be written as a line"},
	{"   ", false, "nor a blank one — it refuses the client's whole config"},
	{"<>", false, "empty tag"},
	{"<", false, "a lone bracket"},
	{">", false, "the kernel reads this as zero tags"},
}

func TestPortableIField(t *testing.T) {
	for _, tc := range portableIFieldCases {
		t.Run(tc.in, func(t *testing.T) {
			if got := PortableIField(tc.in); got != tc.want {
				t.Errorf("PortableIField(%q) = %v, want %v — %s", tc.in, got, tc.want, tc.why)
			}
		})
	}
}

// Whatever the generator can build must survive the predicate, or the panel
// would refuse to export its own output.
func TestPortableIField_AcceptsWhatWeGenerate(t *testing.T) {
	for _, v := range []string{
		"<b 0x16030101430100013f0303><r 32><b 0x20><r 32>",
		"<b 0x1403030001011703030030><r 48>",
		"<r 103>",
		"<rc 16>",
		"<rd 8>",
		"<t>",
	} {
		if !PortableIField(v) {
			t.Errorf("the panel's own generator emits %q and the predicate refuses it", v)
		}
	}
}
