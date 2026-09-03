// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"strings"
	"testing"
)

func TestRenameAwgInterfaceSeq(t *testing.T) {
	var got []string
	set := func(args ...string) error {
		got = append(got, strings.Join(args, " "))
		return nil
	}
	if err := renameAwgInterfaceSeq(set, "awg0", "awg1"); err != nil {
		t.Fatal(err)
	}
	want := []string{"awg0 down", "awg0 name awg1", "awg1 up"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRenameAwgInterfaceSeq_RenameFailsRestoresUp(t *testing.T) {
	var got []string
	set := func(args ...string) error {
		s := strings.Join(args, " ")
		got = append(got, s)
		if strings.Contains(s, "name") {
			return fmt.Errorf("busy")
		}
		return nil
	}
	if err := renameAwgInterfaceSeq(set, "awg0", "awg1"); err == nil {
		t.Fatal("want error")
	}
	want := []string{"awg0 down", "awg0 name awg1", "awg0 up"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("got %v want %v", got, want)
	}
}

func TestRenameAwgInterfaceSeq_SameName(t *testing.T) {
	calls := 0
	set := func(...string) error { calls++; return nil }
	if err := renameAwgInterfaceSeq(set, "awg1", "awg1"); err != nil || calls != 0 {
		t.Fatalf("err=%v calls=%d", err, calls)
	}
}
