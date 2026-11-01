package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io/fs"
	"strings"
	"syscall"
	"testing"
)

func TestInstallPackageRejectsUnsafeEntrypoint(t *testing.T) {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	manifest, _ := zw.Create("manifest.json")
	_ = json.NewEncoder(manifest).Encode(Manifest{ID: "demo", Name: "Demo", Version: "1.0.0", APIVersion: 1, Type: "sidecar", Entrypoint: "../run", Architectures: []string{"amd64"}})
	_ = zw.Close()
	if _, err := InstallPackage(bytes.NewReader(b.Bytes()), int64(b.Len())); err == nil {
		t.Fatal("expected unsafe entrypoint rejection")
	}
}

func TestValidateManifestRequiresSidecarV1(t *testing.T) {
	if err := validateManifest(Manifest{ID: "demo", Name: "Demo", Version: "1", APIVersion: 2, Type: "sidecar", Entrypoint: "run"}); err == nil {
		t.Fatal("expected API version rejection")
	}
}

func TestRejectUnsafePluginIDs(t *testing.T) {
	for _, id := range []string{"..", "../outside", "", "a/b", "A", "a\\b"} {
		if err := validateID(id); err == nil {
			t.Errorf("accepted unsafe ID %q", id)
		}
		if err := Remove(id); err == nil {
			t.Errorf("Remove accepted unsafe ID %q", id)
		}
	}
}

func TestValidatePermissions(t *testing.T) {
	good := Manifest{Permissions: map[string][]string{"network": {networkNone}, "panelApi": {}, "bot": {}}}
	if err := validatePermissions(good.Permissions); err != nil {
		t.Fatalf("rejected valid Stage 2 permissions: %v", err)
	}
	legacy := Manifest{Permissions: map[string][]string{"panelApi": {"*"}}}
	if err := validatePermissions(legacy.Permissions); err == nil {
		t.Fatal("accepted panelApi permission without a seam")
	}
	bot := Manifest{Permissions: map[string][]string{"bot": {"token"}}}
	if err := validatePermissions(bot.Permissions); err == nil {
		t.Fatal("accepted bot permission without a seam")
	}
	unknown := Manifest{Permissions: map[string][]string{"fs": {"rw"}}}
	if err := validatePermissions(unknown.Permissions); err == nil {
		t.Fatal("accepted unknown permission kind")
	}
	badToken := Manifest{Permissions: map[string][]string{"network": {"allow:10.0.0.0/8"}}}
	if err := validatePermissions(badToken.Permissions); err == nil {
		t.Fatal("accepted an unimplemented network rule")
	}
}

func TestValidateHealth(t *testing.T) {
	isolated := Manifest{Health: &HealthCheck{Type: "http", Target: "http://127.0.0.1:80/healthz"}}
	if err := validateManifest(isolated); err == nil {
		t.Fatal("accepted a probe against a network-isolated plugin")
	}
	h := &HealthCheck{Type: "http", Target: "http://127.0.0.1:80/healthz"}
	if err := validateHealth(h, false); err != nil {
		t.Fatalf("rejected a valid http probe: %v", err)
	}
	if h.IntervalSeconds != 30 || h.TimeoutSeconds != 5 {
		t.Fatalf("probe defaults not applied: %+v", h)
	}
	if err := validateHealth(&HealthCheck{Type: "grpc", Target: "x"}, false); err == nil {
		t.Fatal("accepted unknown probe type")
	}
	if err := validateHealth(&HealthCheck{Type: "tcp", Target: "127.0.0.1:notaport"}, false); err == nil {
		t.Fatal("accepted invalid tcp target")
	}
	slow := &HealthCheck{Type: "tcp", Target: "127.0.0.1:80", IntervalSeconds: 10, TimeoutSeconds: 30}
	if err := validateHealth(slow, false); err == nil {
		t.Fatal("accepted probe timeout above interval")
	}
}

func TestStartFailureMapsEPERM(t *testing.T) {
	m := Manifest{Permissions: map[string][]string{"network": {networkNone}}}
	err := &fs.PathError{Op: "fork/exec", Path: "/bin/sh", Err: syscall.EPERM}
	got := startFailure(err, m)
	if !strings.Contains(got, "sandbox") || !strings.Contains(got, "fail closed") {
		t.Fatalf("EPERM not mapped for an isolation request: %v", got)
	}
	plain := startFailure(err, Manifest{})
	if plain != err.Error() {
		t.Fatalf("plain host must keep the raw error: %v", plain)
	}
}

func TestEnforcementRequestIsValid(t *testing.T) {
	m := Manifest{ID: "demo", Name: "Demo", Version: "1.0.0", APIVersion: 1, Type: "sidecar", Entrypoint: "run.sh", Permissions: map[string][]string{"network": {networkNone}, "panelApi": {}, "bot": {}}, Resources: &Resources{MaxMemoryMB: 64}}
	if !needsEnforcement(m) || !denyNetwork(m) || memLimit(m) != 64 {
		t.Fatalf("enforcement flags lost: %+v", m)
	}
	if err := validateManifest(m); err != nil {
		t.Fatalf("valid manifest rejected: %v", err)
	}
	oversized := Manifest{ID: "demo", Name: "Demo", Version: "1.0.0", APIVersion: 1, Type: "sidecar", Entrypoint: "run.sh", Resources: &Resources{MaxMemoryMB: 8}}
	if err := validateManifest(oversized); err == nil {
		t.Fatal("accepted a too-small memory limit")
	}
}
