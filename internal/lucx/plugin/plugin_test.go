package plugin

import (
	"archive/zip"
	"bytes"
	"encoding/json"
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
