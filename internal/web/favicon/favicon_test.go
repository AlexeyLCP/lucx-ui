package favicon

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestHref_Empty(t *testing.T) {
	href, err := Href("  ")
	if err != nil || href != "" {
		t.Fatalf("Href(empty) = %q, %v", href, err)
	}
}

func TestHref_Emoji(t *testing.T) {
	href, err := Href("🐰")
	if err != nil {
		t.Fatal(err)
	}
	const prefix = "data:image/svg+xml;base64,"
	if !strings.HasPrefix(href, prefix) {
		t.Fatalf("href = %q", href)
	}
	raw, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(href, prefix))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "🐰") {
		t.Fatalf("svg = %s", raw)
	}
}

func TestHref_RawBase64AndLinkTag(t *testing.T) {
	svg := `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 100 100"><text y=".9em" font-size="90">🐰</text></svg>`
	b64 := base64.StdEncoding.EncodeToString([]byte(svg))
	href, err := Href(b64)
	if err != nil {
		t.Fatal(err)
	}
	if href != "data:image/svg+xml;base64,"+b64 {
		t.Fatalf("href = %q", href)
	}
	link := `<link rel="icon" href="` + href + `">`
	got, err := Href(link)
	if err != nil || got != href {
		t.Fatalf("Href(link) = %q, %v", got, err)
	}
	if tag := LinkTag("🐰"); !strings.Contains(tag, `rel="icon"`) {
		t.Fatalf("LinkTag = %q", tag)
	}
}

func TestHref_RejectsScript(t *testing.T) {
	if _, err := Href("javascript:alert(1)"); err == nil {
		t.Fatal("expected error")
	}
	if _, err := Href("data:text/html;base64,PHNjcmlwdD4="); err == nil {
		t.Fatal("expected error")
	}
}
