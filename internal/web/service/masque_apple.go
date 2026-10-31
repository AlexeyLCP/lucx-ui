package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// AppleRelayExportRequest describes the operator-controlled parts of Apple's
// Network Relay declaration. It is a DDM settings document, not a mobileconfig.
type AppleRelayExportRequest struct {
	Name             string   `json:"name"`
	RelayURL         string   `json:"relayUrl"`
	HTTP2RelayURL    string   `json:"http2RelayUrl"`
	HTTP3RelayURL    string   `json:"http3RelayUrl"`
	MatchDomains     []string `json:"matchDomains"`
	ExcludedDomains  []string `json:"excludedDomains"`
	MatchFQDNs       []string `json:"matchFQDNs"`
	ExcludedFQDNs    []string `json:"excludedFQDNs"`
	PublicKeyAssets  []string `json:"publicKeyAssets"`
	AllowDNSFailover bool     `json:"allowDNSFailover"`
	UIEnabled        bool     `json:"uiEnabled"`
	RelayUUID        string   `json:"relayUUID"`
}

type AppleRelayDDM struct {
	Format        string                `json:"format"`
	Compatibility string                `json:"compatibility"`
	Warning       string                `json:"warning"`
	Declaration   AppleRelayDeclaration `json:"declaration"`
}

type AppleRelayDeclaration struct {
	Type       string            `json:"type"`
	Identifier string            `json:"identifier"`
	Payload    AppleRelayPayload `json:"payload"`
}

type AppleRelayPayload struct {
	Name             string       `json:"Name"`
	Relays           []AppleRelay `json:"Relays"`
	MatchDomains     []string     `json:"MatchDomains,omitempty"`
	ExcludedDomains  []string     `json:"ExcludedDomains,omitempty"`
	MatchFQDNs       []string     `json:"MatchFQDNs,omitempty"`
	ExcludedFQDNs    []string     `json:"ExcludedFQDNs,omitempty"`
	RelayUUID        string       `json:"RelayUUID,omitempty"`
	UIEnabled        bool         `json:"UIEnabled,omitempty"`
	AllowDNSFailover bool         `json:"AllowDNSFailover,omitempty"`
}

type AppleRelay struct {
	HTTP2RelayURL   string   `json:"HTTP2RelayURL,omitempty"`
	HTTP3RelayURL   string   `json:"HTTP3RelayURL,omitempty"`
	PublicKeyAssets []string `json:"PublicKeyAssets,omitempty"`
}

func BuildAppleRelayDDM(inbound *model.Inbound, host string, req AppleRelayExportRequest) (AppleRelayDDM, error) {
	if inbound == nil || inbound.Protocol != model.MASQUE {
		return AppleRelayDDM{}, errors.New("apple relay export requires a MASQUE inbound")
	}
	if !inbound.Enable {
		return AppleRelayDDM{}, errors.New("apple relay export requires an enabled MASQUE inbound")
	}
	host = strings.TrimSpace(host)
	if host == "" {
		return AppleRelayDDM{}, errors.New("apple relay export requires a public host")
	}
	settings := struct {
		Path string `json:"path"`
	}{}
	_ = json.Unmarshal([]byte(inbound.StreamSettings), &settings)
	path := strings.TrimSpace(settings.Path)
	if path == "" {
		path = "/.well-known/masque/ip/*/*/"
	}
	if req.Name == "" {
		req.Name = strings.TrimSpace(inbound.Remark)
	}
	if req.Name == "" {
		req.Name = "LucX MASQUE Relay"
	}
	if req.RelayURL != "" && req.HTTP2RelayURL == "" && req.HTTP3RelayURL == "" {
		req.HTTP3RelayURL = req.RelayURL
	}
	if req.HTTP2RelayURL == "" && req.HTTP3RelayURL == "" {
		req.HTTP3RelayURL = fmt.Sprintf("https://%s:%d%s", host, inbound.Port, path)
	}
	for _, relayURL := range []string{req.HTTP2RelayURL, req.HTTP3RelayURL} {
		if relayURL == "" {
			continue
		}
		u, err := url.Parse(relayURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
			return AppleRelayDDM{}, fmt.Errorf("invalid relay URL %q", relayURL)
		}
	}
	if req.RelayUUID == "" {
		seed := sha256.Sum256([]byte(fmt.Sprintf("%d:%s:%d", inbound.Id, host, inbound.Port)))
		req.RelayUUID = uuid.NewSHA1(uuid.NameSpaceURL, []byte(hex.EncodeToString(seed[:]))).String()
	}
	if _, err := uuid.Parse(req.RelayUUID); err != nil {
		return AppleRelayDDM{}, errors.New("relayUUID must be a UUID")
	}
	name := fmt.Sprintf("lucx.networkrelay.%d", inbound.Id)
	return AppleRelayDDM{
		Format:        "apple-network-relay-ddm",
		Compatibility: "Apple Network Relay declarative configuration; minimum OS iOS/iPadOS/macOS/visionOS 27",
		Warning:       "This is a DDM settings document, not a standalone .mobileconfig. Submit it through a compatible Apple MDM/DDM service and verify device behavior.",
		Declaration: AppleRelayDeclaration{
			Type:       "com.apple.configuration.networkrelay",
			Identifier: name,
			Payload: AppleRelayPayload{
				Name: req.Name, RelayUUID: req.RelayUUID, UIEnabled: req.UIEnabled,
				AllowDNSFailover: req.AllowDNSFailover, MatchDomains: req.MatchDomains,
				ExcludedDomains: req.ExcludedDomains, MatchFQDNs: req.MatchFQDNs,
				ExcludedFQDNs: req.ExcludedFQDNs, Relays: []AppleRelay{{HTTP2RelayURL: req.HTTP2RelayURL, HTTP3RelayURL: req.HTTP3RelayURL, PublicKeyAssets: req.PublicKeyAssets}},
			},
		},
	}, nil
}

func AppleRelayInboundID(raw string) (int, error) { return strconv.Atoi(strings.TrimSpace(raw)) }
