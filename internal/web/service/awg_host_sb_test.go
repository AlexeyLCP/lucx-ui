package service

import (
	"os"
	"strings"
	"testing"
)

func TestFirstNonEmptyLogLineSkipsAnsiAndBlank(t *testing.T) {
	out := "\x1b[31m┌───┐\x1b[0m\n\n \x1b[1;31mERROR: Secure Boot включён\x1b[0m\n"
	if got := firstNonEmptyLogLine(out); got != "┌───┐" {
		t.Fatalf("first line = %q", got)
	}
	if got := firstNonEmptyLogLine("   \n\t \n"); got != "script exited non-zero" {
		t.Fatalf("blank output = %q", got)
	}
	long := strings.Repeat("a", 300)
	if got := firstNonEmptyLogLine(long); len(got) != 160 {
		t.Fatalf("long line kept %d bytes", len(got))
	}
}

// The install script must refuse to finish when Secure Boot rejected the
// module: writing the loaded-marker or the reboot flag there made every later
// x-ui update treat the host as pinned while nothing works.
func TestInstallAwgModuleScriptHasSecureBootGate(t *testing.T) {
	b, err := os.ReadFile("../../../bin/install-awg-module.sh")
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	for _, want := range []string{
		"awg_sb_enabled()",
		"awg_sb_error",
		"exit 1",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("install-awg-module.sh lost the Secure Boot gate (%s)", want)
		}
	}
	// The exit must sit between the modprobe/lsmod checks and the final
	// success echo, i.e. the script ends with the echo only on success.
	if idx := strings.Index(got, "=== Установка AWG завершена ==="); strings.LastIndex(got, "exit 1") > idx {
		t.Fatal("Secure Boot gate must precede the completion echo")
	}
}
