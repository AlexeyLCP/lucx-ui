// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package parser

import (
	"testing"
)

func TestParseSSHOutput_FullOutput(t *testing.T) {
	input := `═══════════════════════════════════════════
     Panel Installation Complete!
═══════════════════════════════════════════
Username:    admin12345
Password:    xK9mP2vL7q
Port:        2053
WebBasePath: /aB3dEfGhIjKlMnOpQr
Access URL:  https://5.9.1.2:2053/aB3dEfGhIjKlMnOpQr
API Token:   eyJhbGciOiJIUzI1NiJ9.test
═══════════════════════════════════════════
⚠ IMPORTANT: Save these credentials securely!
⚠ SSL Certificate: Enabled and configured`

	creds, err := ParseSSHOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Scheme != "https" {
		t.Errorf("expected scheme 'https', got '%s'", creds.Scheme)
	}
	if creds.Host != "5.9.1.2" {
		t.Errorf("expected host '5.9.1.2', got '%s'", creds.Host)
	}
	if creds.Port != 2053 {
		t.Errorf("expected port 2053, got %d", creds.Port)
	}
	if creds.WebBasePath != "/aB3dEfGhIjKlMnOpQr" {
		t.Errorf("unexpected WebBasePath: %s", creds.WebBasePath)
	}
	if creds.Username != "admin12345" {
		t.Errorf("expected username 'admin12345', got '%s'", creds.Username)
	}
	if creds.Password != "xK9mP2vL7q" {
		t.Errorf("expected password 'xK9mP2vL7q', got '%s'", creds.Password)
	}
	if creds.APIToken != "eyJhbGciOiJIUzI1NiJ9.test" {
		t.Errorf("unexpected API token: %s", creds.APIToken)
	}
}

func TestParseSSHOutput_HTTPNoSSL(t *testing.T) {
	input := `═══════════════════════════════════════════
     Panel Installation Complete!
═══════════════════════════════════════════
Username:    user
Password:    pass
Port:        8080
WebBasePath: /mypanel
Access URL:  http://10.0.0.1:8080/mypanel
API Token:   tok123
═══════════════════════════════════════════
⚠ SSL Certificate: Skipped — panel is HTTP-only.`

	creds, err := ParseSSHOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Scheme != "http" {
		t.Errorf("expected scheme 'http', got '%s'", creds.Scheme)
	}
	if creds.Port != 8080 {
		t.Errorf("expected port 8080, got %d", creds.Port)
	}
}

func TestParseSSHOutput_Localhost(t *testing.T) {
	input := `Access URL:  https://127.0.0.1:2053/secretPath
Username:    admin
Password:    pw12345
API Token:   tok`

	creds, err := ParseSSHOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Host != "127.0.0.1" {
		t.Errorf("expected host '127.0.0.1', got '%s'", creds.Host)
	}
}

func TestParseSSHOutput_EmptyInput(t *testing.T) {
	_, err := ParseSSHOutput("")
	if err == nil {
		t.Fatal("expected error for empty input")
	}
}

func TestParseSSHOutput_GarbageInput(t *testing.T) {
	_, err := ParseSSHOutput("some random text\nnothing useful here")
	if err == nil {
		t.Fatal("expected error for unrecognizable input")
	}
}


func TestParseSSHOutput_ANSIColors(t *testing.T) {
	// Simulates actual SSH output with ANSI color codes from install script
	input := "\033[0;32mUsername:    admin12345\033[0m\n" +
		"\033[0;32mPassword:    xK9mP2vL7q\033[0m\n" +
		"\033[0;32mPort:        2053\033[0m\n" +
		"\033[0;32mAccess URL:  https://5.9.1.2:2053/test\033[0m\n" +
		"\033[0;32mAPI Token:   tok123\033[0m"

	creds, err := ParseSSHOutput(input)
	if err != nil {
		t.Fatalf("ANSI colors should not break parsing: %v", err)
	}
	if creds.Username != "admin12345" {
		t.Errorf("expected admin12345, got %s", creds.Username)
	}
	if creds.Password != "xK9mP2vL7q" {
		t.Errorf("expected xK9mP2vL7q, got %s", creds.Password)
	}
	if creds.APIToken != "tok123" {
		t.Errorf("expected tok123, got %s", creds.APIToken)
	}
}
func TestParseSSHOutput_PartialOutput_URLOnly(t *testing.T) {
	input := `Access URL:  https://5.9.1.2:8443/mybase
Username:    myuser`

	creds, err := ParseSSHOutput(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if creds.Host != "5.9.1.2" {
		t.Errorf("expected host '5.9.1.2', got '%s'", creds.Host)
	}
	if creds.Username != "myuser" {
		t.Errorf("expected username 'myuser', got '%s'", creds.Username)
	}
	if creds.Password != "" {
		t.Errorf("expected empty password, got '%s'", creds.Password)
	}
}
