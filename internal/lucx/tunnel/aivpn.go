// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

// AIVPN sidecar: infosave2007/aivpn aivpn-server (MIT) — an anti-DPI UDP VPN
// gateway whose traffic is masked as STUN / QUIC v1 (HTTP/3 CONNECT-UDP) /
// ICE media flows. The core is the sole authority for client credentials: it
// owns clients.json and mints the `aivpn://` connection key per client; the
// panel only drives the local management HTTP API over a unix socket and
// re-displays the server-issued key. The panel never generates or rotates
// server or client key material (Rule 0).

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const (
	// aivpnDefaultListen mirrors the upstream default port family; the
	// inbound row carries the actual UDP port.
	aivpnDefaultHost = "0.0.0.0"
	// aivpnDefaultIPv6Prefix is the upstream sample ULA prefix.
	aivpnDefaultIPv6Prefix = "fd10:aivpn::/48"
)

// AivpnClient is the panel-facing row of one aivpn client: the server-issued
// connection key plus the counters the management API reports.
type AivpnClient struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	Key           string `json:"key,omitempty"`
	BytesIn       int64  `json:"bytesIn"`
	BytesOut      int64  `json:"bytesOut"`
	LastConnected int64  `json:"lastConnected,omitempty"`
}

// AivpnAPI is the local management HTTP client over the core's unix socket
// (Crate feature management-api: present in every released server binary).
type AivpnAPI struct {
	socket string
	hc     *http.Client
}

func NewAivpnAPI(socket string) *AivpnAPI {
	return &AivpnAPI{
		socket: socket,
		hc: &http.Client{
			Timeout: 15 * time.Second,
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					var d net.Dialer
					return d.DialContext(ctx, "unix", socket)
				},
			},
		},
	}
}

func (a *AivpnAPI) do(ctx context.Context, method, path string, in, out any) error {
	var body *strings.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = strings.NewReader(string(raw))
	} else {
		body = strings.NewReader("")
	}
	req, err := http.NewRequestWithContext(ctx, method, "http://aivpn"+path, body)
	if err != nil {
		return err
	}
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := a.hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("AIVPN API %s %s returned HTTP %d", method, path, resp.StatusCode)
	}
	if out != nil {
		return json.NewDecoder(resp.Body).Decode(out)
	}
	return nil
}

// Status returns the raw status payload; a successful call means the core is
// alive and its management API answers.
func (a *AivpnAPI) Status(ctx context.Context) (map[string]any, error) {
	out := map[string]any{}
	if err := a.do(ctx, http.MethodGet, "/api/v1/status", nil, &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ListClients returns every registered client (enabled/disabled, with the
// counters the server reports). The key needs a per-client GET — list only.
func (a *AivpnAPI) ListClients(ctx context.Context) ([]AivpnClient, error) {
	var rows []map[string]any
	if err := a.do(ctx, http.MethodGet, "/api/v1/clients", nil, &rows); err != nil {
		if !a.retryAsWrapper(ctx, &rows) {
			return nil, err
		}
	}
	out := make([]AivpnClient, 0, len(rows))
	for _, m := range rows {
		out = append(out, aivpnClientFromMap(m))
	}
	return out, nil
}

func (a *AivpnAPI) retryAsWrapper(ctx context.Context, rows *[]map[string]any) bool {
	var wrapper map[string]any
	ctx2, cancel := context.WithTimeout(ctx, a.hc.Timeout)
	defer cancel()
	if err := a.do(ctx2, http.MethodGet, "/api/v1/clients", nil, &wrapper); err != nil {
		return false
	}
	for _, key := range []string{"clients", "data", "items"} {
		if list, ok := wrapper[key].([]any); ok {
			for _, item := range list {
				if m, ok := item.(map[string]any); ok {
					*rows = append(*rows, m)
				}
			}
			return true
		}
	}
	return false
}

// AddClient creates a client server-side and returns the issued row (the
// connection key is minted by the server, never by the panel).
func (a *AivpnAPI) AddClient(ctx context.Context, name string) (AivpnClient, error) {
	var raw map[string]any
	if err := a.do(ctx, http.MethodPost, "/api/v1/clients", map[string]any{"name": name}, &raw); err != nil {
		return AivpnClient{}, err
	}
	client := aivpnClientFromMap(raw)
	if client.Key == "" {
		// The key may live in a wrapper object ("connection" / "bootstrap").
		for _, wrap := range []string{"connection_key", "connection", "bootstrap", "client"} {
			if sub, ok := raw[wrap].(map[string]any); ok {
				client = aivpnClientFromMap(sub)
				break
			}
			if s, ok := raw[wrap].(string); ok && strings.HasPrefix(s, "aivpn://") {
				client.Key = s
				break
			}
		}
	}
	if client.Name == "" {
		client.Name = name
	}
	return client, nil
}

// ClientKey re-reads the issued connection key of one client (reprint path).
// v1.2.0 (InitialV2) mint-strips every list/single payload and serves the key
// from a dedicated endpoint; pre-v1.2.0 cores rode it inside the client
// payload — that stays the fallback.
func (a *AivpnAPI) ClientKey(ctx context.Context, id string) (string, error) {
	path := "/api/v1/clients/" + aivpnPathID(id)
	var wrapper map[string]any
	if err := a.do(ctx, http.MethodGet, path+"/connection-key", nil, &wrapper); err == nil {
		if key := strings.TrimSpace(aivpnStr(wrapper, "connection_key", "connectionKey", "key")); strings.HasPrefix(key, "aivpn://") {
			return key, nil
		}
	}
	var raw map[string]any
	if err := a.do(ctx, http.MethodGet, path, nil, &raw); err != nil {
		return "", err
	}
	client := aivpnClientFromMap(raw)
	if client.Key == "" {
		for _, wrap := range []string{"connection_key", "connection", "bootstrap", "client"} {
			if sub, ok := raw[wrap].(map[string]any); ok {
				client = aivpnClientFromMap(sub)
				break
			}
			if s, ok := raw[wrap].(string); ok && strings.HasPrefix(s, "aivpn://") {
				client = AivpnClient{Key: s}
				break
			}
		}
	}
	if client.Key == "" {
		return "", errors.New("AIVPN API: connection key not present in client payload")
	}
	return client.Key, nil
}

func (a *AivpnAPI) SetEnabled(ctx context.Context, id string, enabled bool) error {
	return a.do(ctx, http.MethodPatch, "/api/v1/clients/"+aivpnPathID(id), map[string]any{"enabled": enabled}, nil)
}

func (a *AivpnAPI) RemoveClient(ctx context.Context, id string) error {
	return a.do(ctx, http.MethodDelete, "/api/v1/clients/"+aivpnPathID(id), nil, nil)
}

func aivpnPathID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || len(id) > 64 {
		return "0"
	}
	for _, r := range id {
		isAlnum := r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
		if !isAlnum && r != '-' && r != '_' {
			return "0"
		}
	}
	return id
}

func aivpnGetFunc(m map[string]any, names ...string) any {
	for _, n := range names {
		if v, ok := m[n]; ok {
			return v
		}
	}
	return nil
}

func aivpnStr(m map[string]any, names ...string) string {
	switch v := aivpnGetFunc(m, names...).(type) {
	case string:
		return v
	case float64:
		return strconv.FormatInt(int64(v), 10)
	default:
		return ""
	}
}

func aivpnBool(m map[string]any, names ...string) bool {
	switch v := aivpnGetFunc(m, names...).(type) {
	case bool:
		return v
	case string:
		return v == "true" || v == "1"
	default:
		return false
	}
}

func aivpnInt(m map[string]any, names ...string) int64 {
	switch v := aivpnGetFunc(m, names...).(type) {
	case float64:
		return int64(v)
	case string:
		n, _ := strconv.ParseInt(strings.TrimSpace(v), 10, 64)
		return n
	default:
		return 0
	}
}

// aivpnClientFromMap tolerates the upstream field naming (the management API
// shape is verified on the E2E stand; list/wrapper variants are accepted).
// A missing/blank key is normal for list rows — only Add/Get return one.
func aivpnClientFromMap(m map[string]any) AivpnClient {
	if m == nil {
		return AivpnClient{}
	}
	if sub, ok := aivpnGetFunc(m, "client").(map[string]any); ok && len(m) <= 2 {
		if aivpnClientFromMap(sub) != (AivpnClient{}) {
			return aivpnClientFromMap(sub)
		}
	}
	key := aivpnStr(m, "connection_key", "connectionKey", "key", "conn_key")
	if key != "" && !strings.HasPrefix(key, "aivpn://") {
		key = ""
	}
	last := aivpnInt(m, "last_connected", "lastConnected", "last_seen", "lastConnectedAt")
	if last > 1<<62 {
		last /= 1000
	}
	disabled := aivpnBool(m, "disabled")
	return AivpnClient{
		ID:            aivpnStr(m, "id", "client_id", "clientId"),
		Name:          aivpnStr(m, "name", "label"),
		Enabled:       aivpnBool(m, "enabled", "active") || (!disabled && aivpnGetFunc(m, "enabled", "active") == nil),
		Key:           key,
		BytesIn:       aivpnInt(m, "bytes_in", "bytesIn", "download", "rx"),
		BytesOut:      aivpnInt(m, "bytes_out", "bytesOut", "upload", "tx"),
		LastConnected: last,
	}
}

// AivpnConfig is the operator-facing configuration of the single AIVPN
// inbound (one per host). UDP bind port = inbound.Port.
type AivpnConfig struct {
	Remark     string `json:"remark"`
	Enabled    bool   `json:"enabled"`
	ListenHost string `json:"listenHost"`
	// Subnet is the tunnel subnet with the server vpn IP: 10.21.0.1/24.
	// Empty means the id-derived default (stable per inbound; never rewritten).
	Subnet     string `json:"subnet"`
	ServerIP   string `json:"serverIp"`
	TunName    string `json:"tunName,omitempty"`
	MTU        int    `json:"mtu,omitempty"`
	IPv6       bool   `json:"ipv6"`
	IPv6Prefix string `json:"ipv6Prefix,omitempty"`
	ExtraArgs  string `json:"extraArgs,omitempty"`
}

func DefaultAivpnConfig() AivpnConfig {
	return AivpnConfig{ListenHost: aivpnDefaultHost}
}

func (c AivpnConfig) Merge() AivpnConfig {
	if strings.TrimSpace(c.ListenHost) == "" {
		c.ListenHost = aivpnDefaultHost
	}
	c.Subnet = strings.TrimSpace(c.Subnet)
	c.ServerIP = strings.TrimSpace(c.ServerIP)
	c.TunName = strings.TrimSpace(c.TunName)
	if c.MTU < 576 || c.MTU > 1500 {
		c.MTU = 0
	}
	c.IPv6Prefix = strings.TrimSpace(c.IPv6Prefix)
	return c
}

// Validate checks the operator-supplied fields; port comes from the inbound.
func (c AivpnConfig) Validate(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("aivpn: port must be 1-65535")
	}
	if _, _, err := c.serverVPNSuffix(); err != nil {
		return err
	}
	return nil
}

// serverVPNSuffix returns "10.x.0.1", 24 from the stored subnet ("" may not
// pass here — the caller resolves the default first).
func (c AivpnConfig) serverVPNSuffix() (string, int, error) {
	s := c.Subnet
	host, prefix, err := net.ParseCIDR(s)
	if err != nil {
		return "", 0, fmt.Errorf("aivpn: subnet must be CIDR (e.g. 10.21.0.1/24): %w", err)
	}
	ones, _ := prefix.Mask.Size()
	if ones == 0 {
		return "", 0, errors.New("aivpn: subnet prefix length must be set")
	}
	if host.To4() == nil {
		return "", 0, errors.New("aivpn: server vpn ip must be IPv4")
	}
	return host.String(), ones, nil
}

// AivpnDefaultSubnet is the deterministic per-inbound subnet (id-stable, so a
// reconcile can never change an already-issued network config; Rule 0). The
// octet band 200-253 avoids the other panel TUN ranges: CSQTT 10.66.x,
// qWDTT 10.68.x and AWG gateways 10.254.x.
func AivpnDefaultSubnet(id int) string {
	return fmt.Sprintf("10.%d.0.1/24", 200+id%54)
}

// ResolveSubnet returns the stored subnet or the id-derived default.
func (c AivpnConfig) ResolveSubnet(id int) (string, error) {
	s := strings.TrimSpace(c.Subnet)
	if s == "" {
		s = AivpnDefaultSubnet(id)
	}
	check := AivpnConfig{Subnet: s}
	if _, _, err := check.serverVPNSuffix(); err != nil {
		return "", err
	}
	return s, nil
}

// aivpnMasksPath is the shared MaskProfile store the release packs at
// bin/tunnel/aivpn-masks (vendored JSONs from the pinned aivpn release,
// third_party/sidecars/masks). aivpn-server fails to boot without at least
// one mask profile and hands the profiles to clients itself, so nothing is
// copied per client.
func aivpnMasksPath() string {
	return absPath(filepath.Join(workDir(), "aivpn-masks"))
}

// aivpnMasksMissing reports whether the mask store has no .json profile.
func aivpnMasksMissing() bool {
	matches, _ := filepath.Glob(filepath.Join(aivpnMasksPath(), "*.json"))
	return len(matches) == 0
}

// KeyPath is the server X25519 private key file. Minted once; never
// rewritten — the whole client fleet hangs on it (Rule 0).
func aivpnKeyPath(key string) string {
	return absPath(filepath.Join(dataDirFor(key, Aivpn), "server.key"))
}

func aivpnClientsDBPath(key string) string {
	return absPath(filepath.Join(dataDirFor(key, Aivpn), "clients.json"))
}

func aivpnAuditPath(key string) string {
	return absPath(filepath.Join(dataDirFor(key, Aivpn), "audit.log"))
}

// AivpnSocketPath is the management unix socket of one AIVPN instance.
func AivpnSocketPath(key string) string {
	return absPath(filepath.Join(dataDirFor(key, Aivpn), "mgmt.sock"))
}

// EnsureKeyFile creates the server key once. An existing key is never
// touched.
func EnsureAivpnKeyFile(key string) error {
	path := aivpnKeyPath(key)
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}
	if err := mkdirPrivate(dataDirFor(key, Aivpn)); err != nil {
		return err
	}
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o600)
}

// RenderConfig writes the server.json of one AIVPN instance.
func (c AivpnConfig) RenderConfig(key string, port int, subnet string) string {
	cfg := map[string]any{
		"listen_addr":       net.JoinHostPort(c.ListenHost, strconv.Itoa(port)),
		"tun_name":          c.TunName,
		"key_file":          aivpnKeyPath(key),
		"clients_db":        aivpnClientsDBPath(key),
		"audit_log":         aivpnAuditPath(key),
		"mask_dir":          aivpnMasksPath(),
		"log_level":         "info",
		"enable_nat":        true,
		"management_socket": AivpnSocketPath(key),
	}
	if c.MTU > 0 {
		cfg["tun_mtu"] = c.MTU
	}
	host, ones := "10.0.0.1", 24
	check := AivpnConfig{Subnet: subnet}
	if parsed, prefix, err := check.serverVPNSuffix(); err == nil {
		host, ones = parsed, prefix
	}
	network := map[string]any{
		"server_vpn_ip": host,
		"prefix_len":    ones,
		"ipv6_enabled":  c.IPv6,
	}
	if c.MTU > 0 {
		network["mtu"] = c.MTU
	}
	if c.IPv6 {
		prefix := c.IPv6Prefix
		if prefix == "" {
			prefix = aivpnDefaultIPv6Prefix
		}
		network["ipv6_prefix"] = prefix
	}
	cfg["network_config"] = network
	if c.ServerIP != "" {
		cfg["server_ip"] = c.ServerIP
	}
	raw, _ := json.MarshalIndent(cfg, "", "  ")
	return string(raw)
}

func (c AivpnConfig) BuildArgs(key string) []string {
	args := []string{"--config", configPathFor(key, Aivpn)}
	if extra := strings.TrimSpace(c.ExtraArgs); extra != "" {
		args = append(args, strings.Fields(extra)...)
	}
	return args
}

func (c AivpnConfig) String() string {
	return fmt.Sprintf("aivpn subnet=%s server_ip=%s", c.Subnet, c.ServerIP)
}
