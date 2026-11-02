// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

/* Stage 3d: the subscription seam. A plugin whose manifest declares
"sub": ["after"] may append extra links after the client entries of the raw
/sub body. The panel calls the plugin's unix socket (POST /_lucx/sub/after)
with the subscription context and appends the links the plugin returns —
additive only (contract rules 0/0b: issued client entries are never touched,
and expired/depleted info-node responses never see injections). Delivery
borrows the community plugin system's call budget by AllIsA1
<https://github.com/AllIsA1/lucx-plugins>: a short timeout and small bodies,
fail-open on any plugin error. */

const socketSubAfterEndpoint = "/_lucx/sub/after"

const (
	subAfterCallTimeout = 300 * time.Millisecond
	subAfterMaxPerCall  = 32   // links one plugin may contribute
	subAfterMaxTotal    = 64   // links a subscription body may gain
	subAfterMaxLink     = 2048 // bytes of one link line
)

type subAfterRequest struct {
	SubID  string   `json:"subId"`
	Host   string   `json:"host"`
	Count  int      `json:"count"`
	Emails []string `json:"emails"`
}

type subAfterResponse struct {
	Links []string `json:"links"`
}

// needsSocket reports whether the plugin contract hands the plugin a unix
// socket path (events subscribers and sub.after injectors).
func needsSocket(m Manifest) bool {
	return len(m.Events) > 0 || len(m.Permissions["sub"]) > 0
}

// validateSubPermissions gates the subscription seam: "after" is the only
// hook in apiVersion 1 and duplicates carry no meaning.
func validateSubPermissions(rules []string) error {
	seen := map[string]bool{}
	for _, r := range rules {
		if r != "after" {
			return fmt.Errorf("unsupported sub rule %q in apiVersion 1; only \"after\" is implemented", r)
		}
		if seen[r] {
			return fmt.Errorf("sub rule %q is listed twice", r)
		}
		seen[r] = true
	}
	return nil
}

// SubAfterInject is the panel-side hook installed into internal/sub. It
// walks every installed plugin granted sub/after and appends their links.
// Failure of any one plugin never touches the subscription body.
func SubAfterInject(subId, host string, count int, emails []string) []string {
	entries, err := os.ReadDir(root())
	if err != nil {
		return nil
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || len(out) >= subAfterMaxTotal {
			continue
		}
		m, err := Load(e.Name())
		if err != nil || !containsString(m.Permissions["sub"], "after") {
			continue
		}
		links := subAfterLinks(m, subId, host, count, emails)
		if len(out)+len(links) > subAfterMaxTotal {
			links = links[:subAfterMaxTotal-len(out)]
		}
		out = append(out, links...)
	}
	return out
}

// subAfterSocket resolves the plugin socket without exec: the recorded run
// path wins, a live unit runtime directory implies the unit-runner shape,
// everything else is the child-runner shape (also what a plugin outliving a
// panel restart needs).
func subAfterSocket(m Manifest) string {
	if rs := runStateOf(m.ID); rs != nil && rs.socket != "" {
		return rs.socket
	}
	if _, err := os.Stat(filepath.Join("/run", "x-ui-plug-"+m.ID)); err == nil {
		return socketPath(m, runnerUnit)
	}
	return socketPath(m, runnerProcess)
}

func subAfterLinks(m Manifest, subId, host string, count int, emails []string) []string {
	payload, err := json.Marshal(subAfterRequest{SubID: subId, Host: host, Count: count, Emails: emails})
	if err != nil {
		return nil
	}
	socket := subAfterSocket(m)
	var resp subAfterResponse
	if err := postForJSON(socket, socketSubAfterEndpoint, payload, &resp); err != nil {
		if rs := runStateOf(m.ID); rs != nil {
			if n := rs.failed.Add(1); n == 1 {
				_, _ = appendPluginLog(m.ID, "sub/after call failed: "+err.Error())
			}
		}
		return nil
	}
	if rs := runStateOf(m.ID); rs != nil {
		rs.delivered.Add(1)
	}
	out := make([]string, 0, len(resp.Links))
	for _, l := range resp.Links {
		if l = trimLink(l); l == "" || len(l) > subAfterMaxLink {
			continue
		}
		out = append(out, l)
		if len(out) >= subAfterMaxPerCall {
			break
		}
	}
	return out
}

func trimLink(l string) string {
	// json.Unmarshal keeps the raw string; trim like splitLinkLines does.
	for len(l) > 0 && (l[0] == ' ' || l[0] == '\t' || l[0] == '\r' || l[0] == '\n') {
		l = l[1:]
	}
	for len(l) > 0 && (l[len(l)-1] == ' ' || l[len(l)-1] == '\t' || l[len(l)-1] == '\r' || l[len(l)-1] == '\n') {
		l = l[:len(l)-1]
	}
	return l
}

// postForJSON is the seam twin of postToSocket: one-shot POST, short
// window, body bounded to 64 KiB, only 2xx answers decode.
func postForJSON(socket, endpoint string, payload []byte, out any) error {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
	}
	client := &http.Client{Timeout: subAfterCallTimeout, Transport: tr}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://unix"+endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	tr.CloseIdleConnections()
	if err != nil {
		return err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return errors.New("plugin answered " + resp.Status)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("plugin seam answer is not valid JSON: %w", err)
	}
	return nil
}
