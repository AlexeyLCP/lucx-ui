// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package parser

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// NodeCreds holds the extracted connection details from SSH console output.
type NodeCreds struct {
	Scheme      string `json:"scheme"`
	Host        string `json:"host"`
	Port        int    `json:"port"`
	WebBasePath string `json:"webBasePath"`
	Username    string `json:"username"`
	Password    string `json:"password"`
	APIToken    string `json:"apiToken"`
}

// LUCX-HOOK: ANSI escape code stripper for colored SSH output
var reANSI = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string {
	return reANSI.ReplaceAllString(s, "")
}

var (
	reAccessURL  = regexp.MustCompile(`Access URL:\s+(https?)://([^\s:]+)(?::(\d+))?(/\S*)`)
	reUsername   = regexp.MustCompile(`Username:\s+(.+)`)
	rePassword   = regexp.MustCompile(`Password:\s+(.+)`)
	rePort       = regexp.MustCompile(`Port:\s+(\d+)`)
	reWebBase    = regexp.MustCompile(`WebBasePath:\s+(/\S+)`)
	reAPIToken   = regexp.MustCompile(`API Token:\s+(.+)`)
	reSSLEnabled = regexp.MustCompile(`SSL Certificate:\s+Enabled`)
)

// ParseSSHOutput extracts NodeCreds from raw 3x-ui install script output.
func ParseSSHOutput(text string) (*NodeCreds, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("empty input")
	}

	// LUCX-HOOK: Strip ANSI escape codes for robust parsing of colored output
	text = stripANSI(text)

	creds := &NodeCreds{}

	if m := reAccessURL.FindStringSubmatch(text); m != nil {
		creds.Scheme = m[1]
		creds.Host = m[2]
		if m[3] != "" {
			port, _ := strconv.Atoi(m[3])
			creds.Port = port
		}
		creds.WebBasePath = m[4]
	} else {
		if m := rePort.FindStringSubmatch(text); m != nil {
			port, _ := strconv.Atoi(m[1])
			creds.Port = port
		}
		if m := reWebBase.FindStringSubmatch(text); m != nil {
			creds.WebBasePath = m[1]
		}
	}

	if m := reUsername.FindStringSubmatch(text); m != nil {
		creds.Username = strings.TrimSpace(m[1])
	}
	if m := rePassword.FindStringSubmatch(text); m != nil {
		creds.Password = strings.TrimSpace(m[1])
	}
	if m := reAPIToken.FindStringSubmatch(text); m != nil {
		creds.APIToken = strings.TrimSpace(m[1])
	}

	if creds.Scheme == "" {
		if reSSLEnabled.MatchString(text) {
			creds.Scheme = "https"
		} else if creds.Port > 0 {
			creds.Scheme = "http"
		}
	}

	if creds.Host == "" && creds.Port == 0 {
		return nil, fmt.Errorf("could not extract host or port from input")
	}

	return creds, nil
}
