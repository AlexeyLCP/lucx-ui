package panel

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
)

func TestIsNewerVersion(t *testing.T) {
	cases := []struct {
		latest  string
		current string
		want    bool
	}{
		{"v2.9.4", "2.9.3", true},
		{"v2.10.0", "2.9.9", true},
		{"v2.9.3", "2.9.3", false},
		{"v2.9.2", "2.9.3", false},
		{"v3.0.0", "2.9.3", true},
		// LUCX-HOOK: fork suffix comparison. Same upstream base → compare lucx minor.
		{"v3.5.0-lucx.9", "3.5.0-lucx.8", true},  // newer fork release
		{"v3.5.0-lucx.8", "3.5.0-lucx.8", false}, // same fork release
		{"v3.5.0-lucx.7", "3.5.0-lucx.8", false}, // older fork release
		{"v3.5.0", "3.5.0-lucx.8", false},        // plain upstream is older than fork
		{"v3.6.0-lucx.1", "3.5.0-lucx.8", true},  // newer upstream base
	}

	for _, tc := range cases {
		if got := isNewerVersion(tc.latest, tc.current); got != tc.want {
			t.Fatalf("isNewerVersion(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestFilterReleasePage_SkipsOlderAndPrerelease(t *testing.T) {
	current := "v3.6.0-lucx.140"
	releases := []githubRelease{
		{TagName: "dev-latest", Prerelease: true, Body: "rolling"},
		{TagName: "v3.6.0-lucx.145", PublishedAt: "2026-08-20T00:00:00Z", Body: "latest"},
		{TagName: "v3.6.0-lucx.144", PublishedAt: "2026-08-20T00:00:00Z", Body: "mid", Draft: true},
		{TagName: "v3.6.0-lucx.143", PublishedAt: "2026-08-19T00:00:00Z", Body: "skipped"},
		{TagName: "v3.6.0-lucx.140", PublishedAt: "2026-08-18T00:00:00Z", Body: "current"},
		{TagName: "v3.6.0-lucx.139", PublishedAt: "2026-08-17T00:00:00Z", Body: "older"},
	}
	items, hasMore := filterReleasePage(releases, current, 10)
	if hasMore {
		t.Fatal("crossed into current/older — hasMore must be false")
	}
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2 (145 and 143; draft 144 and prerelease skipped)", len(items))
	}
	if items[0].Tag != "v3.6.0-lucx.145" || items[1].Tag != "v3.6.0-lucx.143" {
		t.Fatalf("tags = %s, %s", items[0].Tag, items[1].Tag)
	}
}

func TestFilterReleasePage_HasMoreWhenPageFull(t *testing.T) {
	current := "v3.6.0-lucx.1"
	releases := make([]githubRelease, 10)
	for i := range releases {
		releases[i] = githubRelease{TagName: fmt.Sprintf("v3.6.0-lucx.%d", 20-i), Body: "n"}
	}
	items, hasMore := filterReleasePage(releases, current, 10)
	if !hasMore {
		t.Fatal("full page of newer tags must set hasMore")
	}
	if len(items) != 10 {
		t.Fatalf("got %d items, want 10", len(items))
	}
}

func TestFilterReleasePage_EmptyNilSafe(t *testing.T) {
	items, hasMore := filterReleasePage(nil, "v3.6.0-lucx.1", 10)
	if hasMore || items == nil || len(items) != 0 {
		t.Fatalf("empty page: items=%v hasMore=%v", items, hasMore)
	}
}

func TestCompareVersionStringsRejectsUnexpectedFormats(t *testing.T) {
	if _, ok := compareVersionStrings("latest", "2.9.3"); ok {
		t.Fatal("expected non-semver latest tag to be rejected")
	}
	if _, ok := compareVersionStrings("v2.9", "2.9.3"); ok {
		t.Fatal("expected short version to be rejected")
	}
}

func TestShellQuote(t *testing.T) {
	if got := shellQuote("/usr/bin/curl"); got != "'/usr/bin/curl'" {
		t.Fatalf("unexpected quote result: %s", got)
	}
	if got := shellQuote("/tmp/a'b"); got != "'/tmp/a'\\''b'" {
		t.Fatalf("unexpected quote result with single quote: %s", got)
	}
}

func TestExtractReleaseCommit(t *testing.T) {
	full := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"
	cases := []struct {
		name    string
		release service.Release
		want    string
	}{
		{
			name:    "from body marker",
			release: service.Release{Body: "Rolling build\n\ncommit=" + full + "\nbuilt=2026-06-24T00:00:00Z"},
			want:    full,
		},
		{
			name:    "body marker is case-insensitive and wins over target",
			release: service.Release{Body: "COMMIT=" + full, TargetCommitish: "deadbeef"},
			want:    full,
		},
		{
			name:    "fallback to target commit sha",
			release: service.Release{Body: "no marker here", TargetCommitish: full},
			want:    full,
		},
		{
			name:    "branch target is not a commit",
			release: service.Release{Body: "no marker", TargetCommitish: "main"},
			want:    "",
		},
	}
	for _, tc := range cases {
		if got := extractReleaseCommit(&tc.release); got != tc.want {
			t.Fatalf("%s: extractReleaseCommit = %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestCommitsEqual(t *testing.T) {
	full := "1a2b3c4d5e6f7a8b9c0d1e2f3a4b5c6d7e8f9a0b"
	cases := []struct {
		a, b string
		want bool
	}{
		{"1a2b3c4d", full, true},  // injected 8-char prefix matches full release sha
		{full, "1a2b3c4d", true},  // order independent
		{"1A2B3C4D", full, true},  // case insensitive
		{"deadbeef", full, false}, // different commit
		{"", full, false},         // empty current never matches
		{"1a2b3c4d", "", false},   // empty latest never matches
	}
	for _, tc := range cases {
		if got := commitsEqual(tc.a, tc.b); got != tc.want {
			t.Fatalf("commitsEqual(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestShortCommit(t *testing.T) {
	if got := shortCommit("1a2b3c4d5e6f7a8b"); got != "1a2b3c4d" {
		t.Fatalf("shortCommit truncation = %q, want %q", got, "1a2b3c4d")
	}
	if got := shortCommit("abc"); got != "abc" {
		t.Fatalf("shortCommit short input = %q, want %q", got, "abc")
	}
}

func resetUpdateSlot(t *testing.T) {
	t.Helper()
	t.Cleanup(func() {
		updateMu.Lock()
		updateRunning = false
		updateRunID = 0
		updatePID = 0
		updateMu.Unlock()
	})
}

// writeStatusFile hand-writes the status file in the exact wire format
// update.sh itself produces (a bare printf, not Go's json.Marshal), since
// that's the real cross-language contract this package reads in production.
func writeStatusFile(t *testing.T, path string, runID int64, state string) {
	t.Helper()
	body := fmt.Sprintf(`{"runId":"%d","state":"%s","exitCode":0,"finishedAt":%d}`, runID, state, time.Now().Unix())
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestAcquireUpdateSlot(t *testing.T) {
	resetUpdateSlot(t)

	if !acquireUpdateSlot(1) {
		t.Fatal("first acquire: got false, want true")
	}
	if acquireUpdateSlot(2) {
		t.Fatal("second acquire while first is held: got true, want false")
	}
	releaseUpdateSlot()
	if !acquireUpdateSlot(3) {
		t.Fatal("acquire after release: got false, want true")
	}
	releaseUpdateSlot()
}

func TestAcquireUpdateSlotExpiresAfterStaleWindow(t *testing.T) {
	resetUpdateSlot(t)

	if !acquireUpdateSlot(1) {
		t.Fatal("first acquire: got false, want true")
	}
	updateMu.Lock()
	updateStarted = time.Now().Add(-(updateStaleAfter + time.Second))
	updateMu.Unlock()

	if !acquireUpdateSlot(2) {
		t.Fatal("acquire after stale window elapsed: got false, want true")
	}
	releaseUpdateSlot()
}

// TestAcquireUpdateSlotWaitsForAliveProcessPastStaleWindow is the regression
// test for the concurrency bug an upstream review found: past
// updateStaleAfter, the old logic freed the slot purely on elapsed time, even
// if the process it launched was still genuinely running (not crashed) --
// update.sh's own package-manager step plus several downloads can plausibly
// run long on a slow host with nothing actually wrong. Now a confirmed-alive
// PID keeps the slot held past the stale window.
func TestAcquireUpdateSlotWaitsForAliveProcessPastStaleWindow(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("processAlive is a no-op stub on non-Linux; this test only exercises real liveness checking on Linux")
	}
	resetUpdateSlot(t)

	if !acquireUpdateSlot(1) {
		t.Fatal("first acquire: got false, want true")
	}
	recordUpdatePID(os.Getpid()) // the test process itself: guaranteed alive
	updateMu.Lock()
	updateStarted = time.Now().Add(-(updateStaleAfter + time.Second))
	updateMu.Unlock()

	if acquireUpdateSlot(2) {
		t.Fatal("acquire past the stale window while the recorded PID is still alive: got true, want false")
	}
	releaseUpdateSlot()
}

// TestAcquireUpdateSlotHardCeilingOverridesLiveness confirms the absolute
// backstop: even a confirmed-alive process can't hold the slot forever, so a
// genuinely wedged run can't lock out retries permanently.
func TestAcquireUpdateSlotHardCeilingOverridesLiveness(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("processAlive is a no-op stub on non-Linux; this test only exercises real liveness checking on Linux")
	}
	resetUpdateSlot(t)

	if !acquireUpdateSlot(1) {
		t.Fatal("first acquire: got false, want true")
	}
	recordUpdatePID(os.Getpid())
	updateMu.Lock()
	updateStarted = time.Now().Add(-(updateHardCeiling + time.Second))
	updateMu.Unlock()

	if !acquireUpdateSlot(2) {
		t.Fatal("acquire past the hard ceiling despite a live PID: got false, want true")
	}
	releaseUpdateSlot()
}

// TestAcquireUpdateSlotReleasesOnTerminalStatus is the regression test for the
// bug adversarial review found: a fast failure used to still lock out retries
// for the full updateStaleAfter window, because acquireUpdateSlot only looked
// at the in-memory started-at timestamp, never at the status file's own
// terminal state.
func TestAcquireUpdateSlotReleasesOnTerminalStatus(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	resetUpdateSlot(t)
	path := config.GetUpdateStatusFilePath()

	if !acquireUpdateSlot(111) {
		t.Fatal("first acquire: got false, want true")
	}
	writeStatusFile(t, path, 111, updateStateFailed)

	if !acquireUpdateSlot(222) {
		t.Fatal("acquire after the in-flight run reported failed: got false, want true (should not wait out updateStaleAfter)")
	}
	releaseUpdateSlot()
}

// TestAcquireUpdateSlotIgnoresStaleUnrelatedStatus confirms the terminal-state
// check is scoped to the run it actually launched: a status file left behind
// by some earlier, unrelated run (different runID) must not be mistaken for
// this run finishing.
func TestAcquireUpdateSlotIgnoresStaleUnrelatedStatus(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	resetUpdateSlot(t)
	path := config.GetUpdateStatusFilePath()

	writeStatusFile(t, path, 999, updateStateSuccess)
	if !acquireUpdateSlot(111) {
		t.Fatal("first acquire: got false, want true")
	}

	if acquireUpdateSlot(222) {
		t.Fatal("acquire while status file only reflects an unrelated older runID: got true, want false")
	}
	releaseUpdateSlot()
}

// TestAcquireUpdateSlotConcurrency proves the check-then-set is actually
// atomic under real concurrent access, not just correct when called
// sequentially. A prior version of this test suite only ever called
// acquireUpdateSlot from a single goroutine, so it gave no signal if the
// mutex's core promise (only one concurrent launch wins) were broken.
func TestAcquireUpdateSlotConcurrency(t *testing.T) {
	resetUpdateSlot(t)

	const attempts = 200
	var wins atomic.Int32
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := range attempts {
		go func(runID int64) {
			defer wg.Done()
			if acquireUpdateSlot(runID) {
				wins.Add(1)
			}
		}(int64(i))
	}
	wg.Wait()

	if got := wins.Load(); got != 1 {
		t.Fatalf("concurrent acquireUpdateSlot: %d of %d attempts won, want exactly 1", got, attempts)
	}
	releaseUpdateSlot()
}

func TestGetUpdateStatus(t *testing.T) {
	t.Setenv("XUI_DB_FOLDER", t.TempDir())
	path := config.GetUpdateStatusFilePath()
	svc := &PanelService{}

	if got := svc.GetUpdateStatus(); got.State != updateStatePending {
		t.Fatalf("missing status file: State = %q, want %q", got.State, updateStatePending)
	}

	writeStatusFile(t, path, 1735689600123456789, updateStateSuccess)
	got := svc.GetUpdateStatus()
	if got.RunID != "1735689600123456789" {
		t.Fatalf("RunID = %q, want %q (must round-trip as a decimal string, not a JSON number, or it loses precision past 2^53 in JS)", got.RunID, "1735689600123456789")
	}
	if got.State != updateStateSuccess {
		t.Fatalf("State = %q, want %q", got.State, updateStateSuccess)
	}

	if err := os.WriteFile(path, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := svc.GetUpdateStatus(); got.State != updateStatePending {
		t.Fatalf("corrupt status file: State = %q, want %q", got.State, updateStatePending)
	}

	writeStatusFile(t, path, 1, "some-unrecognized-state")
	if got := svc.GetUpdateStatus(); got.State != updateStatePending {
		t.Fatalf("unrecognized state normalizes to pending: State = %q, want %q", got.State, updateStatePending)
	}
}
