// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package wireguard

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

type Peer struct {
	PublicKey  string          `json:"publicKey"`
	PSK        string          `json:"preSharedKey"`
	AllowedIPs []string        `json:"allowedIPs"`
	KeepAlive  json.RawMessage `json:"keepAlive"`
	Email      string          `json:"email"`
	Enable     *bool           `json:"enable"`
	Keepalive  int             `json:"-"`
}

type Instance struct {
	ID         int
	Tag        string
	Port       int
	Address    string
	MTU        int
	TproxyPort int
	PrivateKey string
	Peers      []Peer
}

func (i Instance) Name() string          { return fmt.Sprintf("lxwg%d", i.ID) }
func (i Instance) Table() int            { return 51000 + i.ID }
func (i Instance) Mark() string          { return fmt.Sprintf("0x%x", 0x50000000+i.ID) }
func (i Instance) FirewallTable() string { return fmt.Sprintf("lxwg_%d", i.ID) }
func (i Instance) DeviceFingerprint() string {
	i.Peers = nil
	b, _ := json.Marshal(i)
	return fmt.Sprintf("%x", sha256.Sum256(b))
}

func IsKernel(ib *model.Inbound) bool {
	if ib == nil || ib.Protocol != model.WireGuard {
		return false
	}
	var s struct {
		Kernel bool `json:"kernel"`
	}
	return json.Unmarshal([]byte(ib.Settings), &s) == nil && s.Kernel
}

func validKey(k string) bool {
	b, err := base64.StdEncoding.DecodeString(k)
	return err == nil && len(b) == 32 && strings.TrimSpace(k) == k
}

func Parse(ib *model.Inbound) (Instance, error) {
	var inst Instance
	if !IsKernel(ib) {
		return inst, fmt.Errorf("kernel WireGuard is not selected")
	}
	var s struct {
		SecretKey  string `json:"secretKey"`
		Address    string `json:"address"`
		MTU        int    `json:"mtu"`
		TproxyPort int    `json:"tproxyPort"`
		Clients    []Peer `json:"clients"`
		Peers      []Peer `json:"peers"`
	}
	if err := json.Unmarshal([]byte(ib.Settings), &s); err != nil {
		return inst, fmt.Errorf("invalid kernel WireGuard settings")
	}
	if ib.Id < 1 || ib.Id > 65535 || ib.Port < 1 || ib.Port > 65535 || !validKey(s.SecretKey) {
		return inst, fmt.Errorf("invalid kernel WireGuard ID, port or private key")
	}
	if ib.NodeID != nil {
		return inst, fmt.Errorf("kernel WireGuard currently supports local Linux inbounds only")
	}
	if ib.Listen != "" && ib.Listen != "0.0.0.0" && ib.Listen != "::" {
		return inst, fmt.Errorf("kernel WireGuard requires a wildcard UDP listen address")
	}
	if s.TproxyPort < 1024 || s.TproxyPort > 65535 || s.TproxyPort == ib.Port {
		return inst, fmt.Errorf("kernel WireGuard requires a distinct TPROXY port in 1024..65535")
	}
	network, err := netip.ParsePrefix(s.Address)
	if err != nil || !network.Addr().Is4() || network.Bits() < 8 || network.Bits() > 30 || network.Addr() == network.Masked().Addr() {
		return inst, fmt.Errorf("kernel WireGuard requires an IPv4 server address with /8../30 subnet")
	}
	if s.MTU == 0 {
		s.MTU = 1420
	}
	if s.MTU < 576 || s.MTU > 9000 {
		return inst, fmt.Errorf("kernel WireGuard MTU must be in 576..9000")
	}
	if len(s.Clients) > 0 && len(s.Peers) > 0 {
		return inst, fmt.Errorf("choose clients or legacy peers before enabling kernel WireGuard")
	}
	peers := s.Clients
	if len(peers) == 0 {
		peers = s.Peers
	}
	disabled := map[string]bool{}
	for _, ct := range ib.ClientStats {
		if !ct.Enable || ct.ExpiryTime > 0 && ct.ExpiryTime <= time.Now().UnixMilli() || ct.Total > 0 && ct.Up+ct.Down >= ct.Total {
			disabled[ct.Email] = true
		}
	}
	keys := map[string]bool{}
	addresses := map[netip.Addr]bool{}
	for _, peer := range peers {
		if peer.Enable != nil && !*peer.Enable || disabled[peer.Email] {
			continue
		}
		if !validKey(peer.PublicKey) || peer.PSK != "" && !validKey(peer.PSK) {
			return inst, fmt.Errorf("invalid kernel WireGuard peer key")
		}
		if keys[peer.PublicKey] {
			return inst, fmt.Errorf("duplicate kernel WireGuard peer key")
		}
		keys[peer.PublicKey] = true
		if len(peer.AllowedIPs) == 0 {
			return inst, fmt.Errorf("kernel WireGuard peers require explicit IPv4 /32 addresses")
		}
		for _, raw := range peer.AllowedIPs {
			address, e := netip.ParsePrefix(raw)
			if e != nil || !address.Addr().Is4() || address.Bits() != 32 || !network.Contains(address.Addr()) || address.Addr() == network.Addr() || address.Addr() == network.Masked().Addr() || addresses[address.Addr()] {
				return inst, fmt.Errorf("kernel WireGuard peer addresses must be unique IPv4 /32 hosts inside the server subnet")
			}
			addresses[address.Addr()] = true
		}
		raw := strings.Trim(string(peer.KeepAlive), "\"")
		if raw != "" && raw != "null" {
			v, e := strconv.Atoi(raw)
			if e != nil || v < 0 || v > 65535 {
				return inst, fmt.Errorf("kernel WireGuard keepalive must be a single integer in 0..65535")
			}
			peer.Keepalive = v
		}
		inst.Peers = append(inst.Peers, peer)
	}
	inst.ID = ib.Id
	inst.Tag = ib.Tag
	inst.Port = ib.Port
	inst.Address = s.Address
	inst.MTU = s.MTU
	inst.TproxyPort = s.TproxyPort
	inst.PrivateKey = s.SecretKey
	return inst, nil
}
