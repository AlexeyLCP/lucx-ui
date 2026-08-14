// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"net"
	"strings"
	"testing"
)

// Every case uses an IP literal so the resolver answers from the literal
// itself and the test never touches the network.
func TestCheckDownloadURL(t *testing.T) {
	tests := []struct {
		name    string
		url     string
		wantErr string
	}{
		{
			name: "public https host is accepted",
			url:  "https://8.8.8.8/lucx-ui/caddy-naive-linux-amd64",
		},
		{
			name:    "plain http is refused",
			url:     "http://8.8.8.8/caddy-naive-linux-amd64",
			wantErr: "must use https",
		},
		{
			name:    "loopback is refused",
			url:     "https://127.0.0.1/caddy-naive-linux-amd64",
			wantErr: "non-public address",
		},
		{
			name:    "IPv6 loopback is refused",
			url:     "https://[::1]/caddy-naive-linux-amd64",
			wantErr: "non-public address",
		},
		{
			name:    "private RFC1918 range is refused",
			url:     "https://10.0.0.5/caddy-naive-linux-amd64",
			wantErr: "non-public address",
		},
		{
			name:    "cloud metadata endpoint is refused",
			url:     "https://169.254.169.254/latest/meta-data/iam/security-credentials/",
			wantErr: "non-public address",
		},
		{
			name:    "carrier-grade NAT range is refused",
			url:     "https://100.64.0.1/caddy-naive-linux-amd64",
			wantErr: "non-public address",
		},
		{
			name:    "file scheme is refused",
			url:     "file:///etc/shadow",
			wantErr: "must use https",
		},
		{
			name:    "url without a host is refused",
			url:     "https:///caddy-naive-linux-amd64",
			wantErr: "no host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := checkDownloadURL(tt.url)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("checkDownloadURL(%q) = %v, want nil", tt.url, err)
				}
				return
			}
			if err == nil {
				t.Fatalf("checkDownloadURL(%q) = nil, want error containing %q", tt.url, tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("checkDownloadURL(%q) = %q, want it to contain %q", tt.url, err, tt.wantErr)
			}
		})
	}
}

func TestIsPublicUnicast(t *testing.T) {
	tests := []struct {
		ip   string
		want bool
	}{
		{"8.8.8.8", true},
		{"1.1.1.1", true},
		{"2606:4700:4700::1111", true},
		{"0.0.0.0", false},
		{"127.0.0.1", false},
		{"10.1.2.3", false},
		{"172.16.0.1", false},
		{"172.31.255.254", false},
		{"192.168.1.1", false},
		{"169.254.169.254", false},
		{"100.64.0.1", false},
		{"100.127.255.255", false},
		{"224.0.0.1", false},
		{"::1", false},
		{"fe80::1", false},
		{"fd00::1", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("net.ParseIP(%q) = nil, the test case itself is malformed", tt.ip)
			}
			if got := isPublicUnicast(ip); got != tt.want {
				t.Fatalf("isPublicUnicast(%s) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// 172.32.0.1 sits just outside RFC1918 and 100.128.0.1 just outside the
// carrier-grade NAT block: both must stay reachable, or a legitimate mirror
// on those ranges would be refused.
func TestIsPublicUnicastBoundaries(t *testing.T) {
	for _, addr := range []string{"172.32.0.1", "100.128.0.1", "9.255.255.255", "11.0.0.1"} {
		t.Run(addr, func(t *testing.T) {
			if !isPublicUnicast(net.ParseIP(addr)) {
				t.Fatalf("isPublicUnicast(%s) = false, want true", addr)
			}
		})
	}
}

func TestIsSHA256Hex(t *testing.T) {
	const valid = "9f86d081884c7d659a2feaa0c55ad015a3bf4f1b2b0b822cd15d6c15b0f00a08"
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{name: "64 lowercase hex characters", in: valid, want: true},
		{name: "empty", in: "", want: false},
		{name: "too short", in: valid[:63], want: false},
		{name: "too long", in: valid + "a", want: false},
		{name: "non-hex character", in: strings.Replace(valid, "9", "z", 1), want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isSHA256Hex(tt.in); got != tt.want {
				t.Fatalf("isSHA256Hex(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

// A malformed checksum must be rejected before anything is fetched, so a typo
// cannot turn into "downloaded, then failed to verify".
func TestDownloadBinaryRejectsMalformedChecksum(t *testing.T) {
	svc := &TunnelService{}
	err := svc.DownloadBinary("https://8.8.8.8/caddy-naive-linux-amd64", "not-a-digest")
	if err == nil {
		t.Fatal("DownloadBinary with a malformed sha256 = nil, want an error")
	}
	if !strings.Contains(err.Error(), "64 hex characters") {
		t.Fatalf("DownloadBinary error = %q, want it to mention the expected length", err)
	}
}

func TestDownloadBinaryRejectsEmptyURL(t *testing.T) {
	svc := &TunnelService{}
	if err := svc.DownloadBinary("   ", ""); err == nil {
		t.Fatal("DownloadBinary with an empty url = nil, want an error")
	}
}
