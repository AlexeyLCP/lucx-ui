// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/awg/vpnuri"
	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/util/common"
	wgutil "github.com/mhsanaei/3x-ui/v3/internal/util/wireguard"
)

// ErrDuplicateOutboundTag is returned when an AWG outbound's Tag collides
// with an existing Xray outbound, AWG outbound, or system tag (direct/block/api).
var ErrDuplicateOutboundTag = errors.New("awg-outbound: duplicate outbound tag")

// AwgOutboundService handles CRUD for client-mode AmneziaWG outbounds.
type AwgOutboundService struct{}

// defaultAwgOutboundSettings generates a fresh client keypair and returns the
// Settings JSON for a new AWG outbound (operator still must fill Address,
// PublicKey, Endpoint upstream-side). PrivateKey is generated via
// x/crypto/curve25519 (same path as defaultAwgClients), not random bytes. Both
// the private and the derived public key are stored so the operator can copy
// the public key when adding this client as a peer on the upstream server.
func defaultAwgOutboundSettings() string {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		return "{}"
	}
	s := map[string]any{
		"privateKey": priv,
		"publicKey":  pub,
		"mtu":        1320,
		"keepalive":  "0",
		"allowedIPs": "0.0.0.0/0, ::/0",
	}
	b, _ := json.Marshal(s)
	return string(b)
}

// checkTagUnique returns ErrDuplicateOutboundTag if tag is already used by
// another AWG outbound (other than ignoreId), matches a system tag, or
// collides with a user-defined Xray outbound tag in the Xray config template.
//
// Full spec compliance would also walk the runtime-injected outbounds (e.g.
// subscription outbounds injected by GetXrayConfig at request time), but those
// are not materialised in the DB/template and would require loading the full
// assembled Xray config (XrayService.GetXrayConfig pulls in inboundService +
// settingService + nodeService + xrayAPI — too invasive for a tag-uniqueness
// guard). The pragmatic cross-check below covers the user-authored template
// outbounds, which is where collisions are most likely to be introduced by an
// operator typing a tag into the AWG outbound form.
//
// KNOWN LIMITATION: a collision with a runtime-injected outbound (one not
// present in the template, only in the assembled config at request time) is
// not caught here and will surface as a "duplicate outbound tag" error on the
// next Xray restart. Per the spec's "loud > silent" philosophy this is
// acceptable: the panel rejects the bad config loudly on restart rather than
// silently misrouting traffic, and the operator can rename the colliding tag.
func checkTagUnique(tag string, ignoreAwgId, ignoreSidecarId int) error {
	if tag == "direct" || tag == "block" || tag == "api" {
		return fmt.Errorf("%w: tag %q is reserved", ErrDuplicateOutboundTag, tag)
	}
	db := database.GetDB()
	var count int64
	if err := db.Model(&model.AwgOutbound{}).
		Where("tag = ? AND id != ?", tag, ignoreAwgId).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: tag %q already used by another AWG outbound", ErrDuplicateOutboundTag, tag)
	}
	count = 0
	if err := db.Model(&model.SidecarOutbound{}).
		Where("tag = ? AND id != ?", tag, ignoreSidecarId).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: tag %q already used by a sidecar outbound", ErrDuplicateOutboundTag, tag)
	}
	// Cross-check against user-authored Xray outbound tags in the config
	// template. Subscription/runtime-injected outbounds are not covered (see
	// the function doc above) — those fail loudly on Xray restart instead.
	if xrayTag, err := tagInXrayTemplate(tag); err != nil {
		return err
	} else if xrayTag {
		return fmt.Errorf("%w: tag %q already used by a user-defined Xray outbound in the template", ErrDuplicateOutboundTag, tag)
	}
	return nil
}

// tagInXrayTemplate reports whether tag appears as an outbound tag in the
// stored Xray config template (xrayTemplateConfig setting). Returns false with
// a nil error when the template cannot be loaded or parsed — we degrade to
// "no cross-check" rather than blocking AWG outbound writes on a malformed
// template, since a malformed template would already break Xray itself on
// the next restart and surface there.
func tagInXrayTemplate(tag string) (bool, error) {
	tmpl, err := (&SettingService{}).GetXrayConfigTemplate()
	if err != nil || tmpl == "" {
		return false, err
	}
	// The template's "outbounds" field is a JSON array of objects; each may
	// carry a "tag" string. Parse just that array rather than unmarshalling
	// the whole config (which would require resolving RawMessage types).
	var wrapper struct {
		Outbounds []map[string]any `json:"outbounds"`
	}
	if err := json.Unmarshal([]byte(tmpl), &wrapper); err != nil {
		return false, nil // malformed template — skip cross-check
	}
	for _, ob := range wrapper.Outbounds {
		if t, ok := ob["tag"].(string); ok && t == tag {
			return true, nil
		}
	}
	return false, nil
}

// checkOutboundIFields rejects an I1-I5 set that would leave awgo-{Id} up but
// unreadable: it applies and passes traffic, yet `awg show` fails with EMSGSIZE.
func checkOutboundIFields(o *model.AwgOutbound) error {
	var s struct {
		I1                  string `json:"i1"`
		I2                  string `json:"i2"`
		I3                  string `json:"i3"`
		I4                  string `json:"i4"`
		I5                  string `json:"i5"`
		HeaderProtectionKey string `json:"headerProtectionKey"`
	}
	if json.Unmarshal([]byte(o.Settings), &s) != nil {
		return nil
	}
	return awg.ValidateIFields("awgo-"+strconv.Itoa(o.Id), s.HeaderProtectionKey, s.I1, s.I2, s.I3, s.I4, s.I5)
}

// AddOutbound persists a new AWG outbound row. If Settings is empty, fills in
// a default keypair via defaultAwgOutboundSettings. Tag uniqueness is enforced.
// When the operator supplied a non-empty Tag it is kept; otherwise the Tag is
// auto-generated as "awgo-{Id}" after the row is created (the Id isn't known
// until after Create) and persisted with an Update.
func (s *AwgOutboundService) AddOutbound(o *model.AwgOutbound) (*model.AwgOutbound, error) {
	tag := strings.TrimSpace(o.Tag)
	if tag == "" {
		// Defer tag uniqueness check until after auto-generation, since the
		// auto-generated "awgo-{Id}" tag is unique by construction (Id is the
		// primary key). We still re-check below for safety in case of races.
		o.Tag = ""
	} else {
		o.Tag = tag
		if err := checkTagUnique(o.Tag, 0, 0); err != nil {
			return nil, err
		}
	}
	if strings.TrimSpace(o.Settings) == "" {
		o.Settings = defaultAwgOutboundSettings()
	}
	if err := checkOutboundIFields(o); err != nil {
		return nil, err
	}
	if err := s.checkSubnetConflict(o); err != nil {
		return nil, err
	}
	db := database.GetDB()
	if err := db.Create(o).Error; err != nil {
		return nil, err
	}
	if o.Tag == "" {
		o.Tag = "awgo-" + strconv.Itoa(o.Id)
		if err := db.Model(o).Update("tag", o.Tag).Error; err != nil {
			return nil, err
		}
		// Re-run uniqueness against the now-final, auto-generated tag. This is
		// almost always a no-op (awgo-{Id} is unique), but guards against a
		// hand-edited DB where another row already holds "awgo-{Id}".
		if err := checkTagUnique(o.Tag, o.Id, 0); err != nil {
			return nil, err
		}
	}
	return o, nil
}

func (s *AwgOutboundService) DelOutbound(id int) error {
	db := database.GetDB()
	return db.Delete(&model.AwgOutbound{}, id).Error
}

func (s *AwgOutboundService) UpdateOutbound(o *model.AwgOutbound) error {
	if err := checkTagUnique(o.Tag, o.Id, 0); err != nil {
		return err
	}
	if err := checkOutboundIFields(o); err != nil {
		return err
	}
	if err := s.checkSubnetConflict(o); err != nil {
		return err
	}
	db := database.GetDB()
	return db.Save(o).Error
}

func (s *AwgOutboundService) SetOutboundEnable(id int, enable bool) error {
	db := database.GetDB()
	return db.Model(&model.AwgOutbound{}).Where("id = ?", id).Update("enable", enable).Error
}

func (s *AwgOutboundService) GetOutbounds() ([]*model.AwgOutbound, error) {
	db := database.GetDB()
	var out []*model.AwgOutbound
	err := db.Order("id ASC").Find(&out).Error
	return out, err
}

// ActiveOutboundTags returns the Tags of all enabled AWG outbounds. The tags
// are injected into the generated Xray config (injectAwgOutbounds) at runtime,
// not into the editable template, so the routing rules UI needs them surfaced
// separately — mirroring how subscription outbound tags are exposed to the
// frontend via xrayResponse["subscriptionOutboundTags"]. Used by the xray
// config controller so the routing-rules outboundTag dropdown can list AWG
// outbound tags alongside template/subscription/reverse tags.
func (s *AwgOutboundService) ActiveOutboundTags() ([]string, error) {
	db := database.GetDB()
	var tags []string
	err := db.Model(&model.AwgOutbound{}).
		Where("enable = ?", true).
		Order("id ASC").
		Pluck("tag", &tags).Error
	return tags, err
}

// ActiveOutboundAddresses returns the tunnel Address of all enabled AWG
// outbounds. Used by defaultAwgClients to avoid allocating a client
// AllowedIPs that collides with an AWG outbound's kernel interface — a
// collision makes the kernel treat the client IP as local (the outbound's
// awgo-N interface owns it), so return-path packets to the client go to lo
// instead of awgN and the client's traffic dies. Caught live on test2:
// client got 10.8.0.2, awgo-2 already had 10.8.0.2 → 0 packets through the
// tunnel. Reporter: tester VladufQa ("создаю второго клиента и у второго
// клиента не идет трафик").
func (s *AwgOutboundService) ActiveOutboundAddresses() ([]string, error) {
	return s.outboundAddresses(true)
}

// outboundAddresses returns the tunnel Address of every AWG outbound row. With
// enabledOnly it restricts to enable=true (the client-allocation collision guard
// only needs live awgo-N interfaces); the subnet-conflict check passes false so
// a currently-disabled outbound cannot silently reintroduce a duplicate
// connected route the moment it is re-enabled (lucx.64).
func (s *AwgOutboundService) outboundAddresses(enabledOnly bool) ([]string, error) {
	db := database.GetDB()
	var rows []string
	// Settings is JSON; the address lives under settings.address. SQLite/MySQL
	// both store Settings as a text blob, so a LIKE scan is portable and the
	// inbound count is tiny (<100). A JSON_EXTRACT would be cleaner but is
	// not portable across sqlite/mysql/postgres.
	q := db.Model(&model.AwgOutbound{})
	if enabledOnly {
		q = q.Where("enable = ?", true)
	}
	err := q.Pluck("settings", &rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rows))
	for _, s := range rows {
		if addr := parseAwgOutboundAddress(s); addr != "" {
			out = append(out, addr)
		}
	}
	return out, nil
}

// parseAwgOutboundAddress extracts the tunnel Address from one AWG outbound
// settings JSON blob. Returns "" when the blob is missing/malformed or has
// no address. Exposed (lowercase, package-private) for unit testing without
// a database — ActiveOutboundAddresses is the DB-backed caller.
func parseAwgOutboundAddress(settings string) string {
	var parsed struct {
		Address string `json:"address"`
	}
	if json.Unmarshal([]byte(settings), &parsed) != nil {
		return ""
	}
	return strings.TrimSpace(parsed.Address)
}

// checkSubnetConflict blocks an AWG outbound whose tunnel Address would install
// a kernel route overlapping an AWG inbound's tunnel subnet or one of its
// clients' addresses. It is the outbound-side mirror of
// InboundService.checkAwgSubnetConflict (which runs when an INBOUND is saved);
// without this direction an operator pasting a provider conf whose tunnel lands
// in a subnet an existing inbound's clients already occupy silently breaks the
// reverse path, and Xray floods "proxy/tun: connection was refused" (lucx.69,
// tester VladufQa: awgo 10.8.0.3 on top of awg2 clients in 10.8.0.0/24). Client
// addresses are checked in addition to the inbound server address because a
// legacy wrong-subnet inbound keeps its clients in a different /24 than its own
// settings.address — comparing only server subnets would miss exactly that case.
// A single-host address (/32 or /128) is exempt: it installs no subnet route.
func (s *AwgOutboundService) checkSubnetConflict(o *model.AwgOutbound) error {
	db := database.GetDB()
	var inbounds []*model.Inbound
	if err := db.Model(model.Inbound{}).Where("protocol = ?", model.AWG).Find(&inbounds).Error; err != nil {
		return err
	}
	return awgOutboundSubnetClash(parseAwgOutboundAddress(o.Settings), inbounds)
}

// awgOutboundSubnetClash is the pure half of checkSubnetConflict — outbound
// tunnel address vs the panel's AWG inbounds — split out so it is unit-testable
// without a database. Returns a descriptive error on the first clash, nil when
// the address is empty/unparseable, a single host, or overlaps nothing.
func awgOutboundSubnetClash(addr string, inbounds []*model.Inbound) error {
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return nil
	}
	oP, err := netip.ParsePrefix(addr)
	if err != nil {
		a, aerr := netip.ParseAddr(addr)
		if aerr != nil {
			return nil
		}
		oP = netip.PrefixFrom(a, a.BitLen())
	}
	oNet := oP.Masked()
	if oNet.Bits() == oNet.Addr().BitLen() {
		return nil
	}
	for _, ib := range inbounds {
		label := ib.Remark
		if label == "" {
			label = ib.Tag
		}
		if ibAddr := awgSettingsAddress(ib.Settings); ibAddr != "" {
			if iP, perr := netip.ParsePrefix(ibAddr); perr == nil && oNet.Overlaps(iP.Masked()) {
				return common.NewError("AWG outbound tunnel", oNet.String(), "conflicts with inbound", label, "tunnel subnet", iP.Masked().String(), "— use a provider conf on a different subnet")
			}
		}
		for _, cip := range awgSettingsClientIPs(ib.Settings) {
			ca, cerr := netip.ParseAddr(cip)
			if cerr != nil {
				continue
			}
			if oNet.Contains(ca) {
				return common.NewError("AWG outbound tunnel", oNet.String(), "conflicts with client address", ca.String(), "of inbound", label, "— use a provider conf on a different subnet")
			}
		}
	}
	return nil
}

func (s *AwgOutboundService) GetOutbound(id int) (*model.AwgOutbound, error) {
	db := database.GetDB()
	o := &model.AwgOutbound{}
	if err := db.First(o, id).Error; err != nil {
		return nil, err
	}
	return o, nil
}

// ParseConf parses a pasted awg-quick .conf and returns ClientSettings, used by
// the "Paste .conf" UI drawer to autofill the form. Also unwraps a vpn:// URI
// or Amnezia JSON envelope (lucx.140 container) down to the inner .conf.
// Accepts configs of any AWG version (1.5/2/3/3.1): it reads every field the
// file carries, including HeaderProtectionKey, AWG3 timers, RandomTrailers and
// DisableCookies, and auto-detects the protocol version from the field set so
// renderClientConf re-emits the same lines. Tolerates whitespace and lines
// without values. Does NOT validate mandatory fields (caller does).
func unwrapOutboundConf(text string) string {
	s := strings.TrimSpace(text)
	if s == "" {
		return text
	}
	if strings.HasPrefix(s, "vpn://") {
		payload, err := vpnuri.Decode(s)
		if err != nil {
			return text
		}
		if conf, err := vpnuri.ConfFromPayload(payload); err == nil {
			return conf
		}
		return text
	}
	if strings.HasPrefix(s, "{") {
		if conf, err := vpnuri.ConfFromPayload([]byte(s)); err == nil {
			return conf
		}
	}
	return text
}

func ParseConf(text string) (awg.ClientSettings, error) {
	text = unwrapOutboundConf(text)
	var s awg.ClientSettings
	section := ""
	for _, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, ";") {
			continue
		}
		if strings.HasPrefix(line, "[") {
			section = strings.ToLower(strings.Trim(line, "[]"))
			continue
		}
		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)
		switch section {
		case "interface":
			switch key {
			case "PrivateKey":
				s.PrivateKey = val
			case "Address":
				s.Address = val
			case "MTU":
				s.MTU, _ = strconv.Atoi(val)
			case "DNS":
				// Provider confs routinely list several nameservers
				// ("DNS = 1.1.1.1, 1.0.0.1") but an AWG outbound carries a single
				// DNS; keep only the first so the form field and any future
				// consumer get one address, not a comma-joined pair (lucx.72,
				// tester VladufQa: "оутбаунд поддерживает только 1 днс, а при
				// импорте вписывается 2 через запятую").
				if first, _, comma := strings.Cut(val, ","); comma {
					s.DNS = strings.TrimSpace(first)
				} else {
					s.DNS = val
				}
			case "Jc":
				s.Jc, _ = strconv.Atoi(val)
			case "Jmin":
				s.Jmin, _ = strconv.Atoi(val)
			case "Jmax":
				s.Jmax, _ = strconv.Atoi(val)
			case "S1":
				s.S1, _ = strconv.Atoi(val)
			case "S2":
				s.S2, _ = strconv.Atoi(val)
			case "S3":
				s.S3, _ = strconv.Atoi(val)
			case "S4":
				s.S4, _ = strconv.Atoi(val)
			case "H1":
				s.H1 = val
			case "H2":
				s.H2 = val
			case "H3":
				s.H3 = val
			case "H4":
				s.H4 = val
			case "I1":
				s.I1 = val
			case "I2":
				s.I2 = val
			case "I3":
				s.I3 = val
			case "I4":
				s.I4 = val
			case "I5":
				s.I5 = val
			case "HeaderProtectionKey":
				s.HeaderProtectionKey = val
			case "ContentPaddingAddition":
				// Device timers/padding are kept verbatim as AwgTimer so AWG3
				// RANGE values ("100-120") survive import instead of being
				// dropped by strconv.Atoi (lucx.74).
				s.ContentPaddingAddition = awg.AwgTimer(val)
			case "RekeyAfterTime":
				s.RekeyAfterTime = awg.AwgTimer(val)
			case "RekeyTimeout":
				s.RekeyTimeout = awg.AwgTimer(val)
			case "RejectAfterTime":
				s.RejectAfterTime = awg.AwgTimer(val)
			case "KeepaliveTimeout":
				s.KeepaliveTimeout = awg.AwgTimer(val)
			case "MaxHandshakeAttempts":
				s.MaxHandshakeAttempts = awg.AwgTimer(val)
			case "RandomTrailers":
				s.RandomTrailers = parseConfBool(val)
			case "DisableCookies":
				s.DisableCookies = parseConfBool(val)
			}
		case "peer":
			switch key {
			case "PublicKey":
				s.PublicKey = val
			case "PresharedKey":
				s.PSK = val
			case "Endpoint":
				s.Endpoint = val
			case "AllowedIPs":
				s.AllowedIPs = val
			case "PersistentKeepalive":
				s.Keepalive = awg.AwgTimer(val)
			}
		}
	}
	// Auto-detect the AWG protocol version from the fields the pasted .conf
	// carried, so a v3 .conf (with HeaderProtectionKey) is stored as version
	// "3" and renderClientConf then re-emits the key. Detection order: HPK →
	// "3"; else S3/S4 or any I1-I5 → "2" (those fields are AWG v2+); else the
	// legacy field set → "1.5". Matches the version-gate logic in
	// awg.NormalizeAWGVersion and the inbound form's version presets.
	switch {
	case s.RandomTrailers || s.DisableCookies:
		s.AwgVersion = "3.1"
	case s.HeaderProtectionKey != "" || !s.ContentPaddingAddition.IsZero() || !s.RekeyAfterTime.IsZero() ||
		!s.RekeyTimeout.IsZero() || !s.RejectAfterTime.IsZero() || !s.KeepaliveTimeout.IsZero() ||
		!s.MaxHandshakeAttempts.IsZero():
		s.AwgVersion = "3"
	case s.S3 != 0 || s.S4 != 0 || s.I1 != "" || s.I2 != "" || s.I3 != "" || s.I4 != "" || s.I5 != "":
		s.AwgVersion = "2"
	default:
		s.AwgVersion = "1.5"
	}
	return s, nil
}

func parseConfBool(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "yes", "1", "on":
		return true
	default:
		return false
	}
}
