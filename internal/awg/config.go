// Copyright (c) 2025 LucX-UI Project.

package awg

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// BuildServerConfig builds the server .conf from inbound.Settings (obfuscation + peers).
// All obfuscation params come from stored settings — single source of truth.
func BuildServerConfig(awg *model.Inbound, upPath, downPath string) string {
	s := parseSettings(awg.Settings)
	if s == nil {
		return ""
	}
	var b strings.Builder

	fmt.Fprintf(&b, "[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", str(s, "privateKey"))
	fmt.Fprintf(&b, "ListenPort = %d\n", awg.Port)
	fmt.Fprintf(&b, "MTU = %d\n", intVal(s, "mtu", 1320))
	fmt.Fprintf(&b, "Jc = %d\n", intVal(s, "jc", 0))
	fmt.Fprintf(&b, "Jmin = %d\n", intVal(s, "jmin", 0))
	fmt.Fprintf(&b, "Jmax = %d\n", intVal(s, "jmax", 0))
	fmt.Fprintf(&b, "S1 = %d\n", intVal(s, "s1", 0))
	fmt.Fprintf(&b, "S2 = %d\n", intVal(s, "s2", 0))
	fmt.Fprintf(&b, "S3 = %d\n", intVal(s, "s3", 0))
	fmt.Fprintf(&b, "S4 = %d\n", intVal(s, "s4", 0))
	fmt.Fprintf(&b, "H1 = %s\n", str(s, "h1"))
	fmt.Fprintf(&b, "H2 = %s\n", str(s, "h2"))
	fmt.Fprintf(&b, "H3 = %s\n", str(s, "h3"))
	fmt.Fprintf(&b, "H4 = %s\n", str(s, "h4"))

	if i1 := str(s, "i1"); i1 != "" {
		fmt.Fprintf(&b, "I1 = <b 0x%s>\n", i1)
		fmt.Fprintf(&b, "I2 = <b 0x%s>\n", str(s, "i2"))
	}
	if i3 := str(s, "i3"); i3 != "" {
		fmt.Fprintf(&b, "I3 = <b 0x%s>\n", i3)
		fmt.Fprintf(&b, "I4 = <b 0x%s>\n", str(s, "i4"))
		fmt.Fprintf(&b, "I5 = <b 0x%s>\n", str(s, "i5"))
	}

	fmt.Fprintf(&b, "PostUp = %s\n", upPath)
	fmt.Fprintf(&b, "PostDown = %s\n", downPath)

	clients := getPeers(awg)
	for _, c := range clients {
		b.WriteString("\n[Peer]\n")
		fmt.Fprintf(&b, "PublicKey = %s\n", c.ID)
		fmt.Fprintf(&b, "PresharedKey = %s\n", c.Password)
		b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
		b.WriteString("PersistentKeepalive = 25\n")
	}

	return b.String()
}

// BuildClientConfig builds a client .conf — uses THE SAME obfuscation as the server.
// serverPubKey: if empty, reads from inbound.Settings["publicKey"].
// clientPrivKey: the client's Curve25519 private key (kept client-side, never stored on server).
// serverAddr: server IP/hostname, endpoint computed as serverAddr:awg.Port.
func BuildClientConfig(awg *model.Inbound, client model.Client, clientPrivKey, serverPubKey, serverAddr string) string {
	s := parseSettings(awg.Settings)
	if s == nil {
		return ""
	}
	// Rule 3: if serverPubKey not passed explicitly, read from inbound.Settings
	if serverPubKey == "" {
		serverPubKey = str(s, "publicKey")
	}
	var b strings.Builder

	fmt.Fprintf(&b, "# %s\n", client.Email)
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", clientPrivKey)
	dns := str(s, "dns")
	if dns == "" {
		dns = "1.1.1.1, 1.0.0.1"
	}
	fmt.Fprintf(&b, "DNS = %s\n", dns)
	mtu := intVal(s, "mtu", 1320)
	fmt.Fprintf(&b, "MTU = %d\n", mtu)

	fmt.Fprintf(&b, "Jc = %d\n", intVal(s, "jc", 0))
	fmt.Fprintf(&b, "Jmin = %d\n", intVal(s, "jmin", 0))
	fmt.Fprintf(&b, "Jmax = %d\n", intVal(s, "jmax", 0))
	fmt.Fprintf(&b, "S1 = %d\n", intVal(s, "s1", 0))
	fmt.Fprintf(&b, "S2 = %d\n", intVal(s, "s2", 0))
	fmt.Fprintf(&b, "S3 = %d\n", intVal(s, "s3", 0))
	fmt.Fprintf(&b, "S4 = %d\n", intVal(s, "s4", 0))
	fmt.Fprintf(&b, "H1 = %s\n", str(s, "h1"))
	fmt.Fprintf(&b, "H2 = %s\n", str(s, "h2"))
	fmt.Fprintf(&b, "H3 = %s\n", str(s, "h3"))
	fmt.Fprintf(&b, "H4 = %s\n", str(s, "h4"))
	if i1 := str(s, "i1"); i1 != "" {
		fmt.Fprintf(&b, "I1 = <b 0x%s>\n", i1)
		fmt.Fprintf(&b, "I2 = <b 0x%s>\n", str(s, "i2"))
	}
	if i3 := str(s, "i3"); i3 != "" {
		fmt.Fprintf(&b, "I3 = <b 0x%s>\n", i3)
		fmt.Fprintf(&b, "I4 = <b 0x%s>\n", str(s, "i4"))
		fmt.Fprintf(&b, "I5 = <b 0x%s>\n", str(s, "i5"))
	}

	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", serverPubKey)
	fmt.Fprintf(&b, "PresharedKey = %s\n", client.Password)
	fmt.Fprintf(&b, "Endpoint = %s:%d\n", serverAddr, awg.Port)
	b.WriteString("AllowedIPs = 0.0.0.0/0, ::/0\n")
	b.WriteString("PersistentKeepalive = 25\n")

	return b.String()
}

// UpdateServerConfig regenerates and writes the server .conf to disk.
// Called after adding/removing clients to keep the file in sync.
func UpdateServerConfig(awg *model.Inbound) error {
	awgID := awg.Id
	s := parseSettings(awg.Settings)
	if s == nil {
		return fmt.Errorf("parse settings")
	}

	upPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-up.sh", awgID))
	downPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d-down.sh", awgID))
	conf := BuildServerConfig(awg, upPath, downPath)
	confPath := filepath.Join(awgConfigDir, fmt.Sprintf("awg%d.conf", awgID))
	if err := os.WriteFile(confPath, []byte(conf), 0600); err != nil {
		return err
	}
	logAWG("UpdateServerConfig: inbound=%d", awgID)
	return nil
}

func parseSettings(raw string) map[string]interface{} {
	var s map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &s); err != nil {
		return nil
	}
	return s
}

func str(s map[string]interface{}, key string) string {
	v, _ := s[key].(string)
	return v
}

func intVal(s map[string]interface{}, key string, def int) int {
	switch v := s[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return def
}

func getPeers(awg *model.Inbound) []model.Client {
	s := parseSettings(awg.Settings)
	if s == nil {
		return nil
	}
	raw, ok := s["clients"]
	if !ok || raw == nil {
		return nil
	}
	arr, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	var peers []model.Client
	for _, item := range arr {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		id, _ := m["id"].(string)
		psk, _ := m["password"].(string)
		enable, _ := m["enable"].(bool)
		if id == "" || psk == "" || !enable {
			continue
		}
		peers = append(peers, model.Client{
			ID:       id,
			Password: psk,
			Enable:   enable,
		})
	}
	return peers
}
