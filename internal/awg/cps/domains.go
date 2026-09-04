// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import "math/rand"

// Region selects which domain pool the CPS generator draws from. "ru" adds
// Russian services (yandex/vk/gosuslugi) on top of the global set; "world"
// sticks to globally reachable front domains.
type Region string

const (
	RegionRU    Region = "ru"
	RegionWorld Region = "world"
)

// MimicryProfile picks the packet shape CPS imitates. TLS = a browser
// ClientHello (Chrome/Firefox/Safari -- see BrowserProfile); DNS = an EDNS0
// query; SIP = a REGISTER; QUIC = a QUIC v1 Initial carrying a TLS ClientHello.
// Ported from pumbaX/awg-multi-script.
type MimicryProfile string

const (
	ProfileTLS  MimicryProfile = "tls"
	ProfileDNS  MimicryProfile = "dns"
	ProfileSIP  MimicryProfile = "sip"
	ProfileQUIC MimicryProfile = "quic"
)

// BrowserProfile selects which browser's TLS ClientHello fingerprint the TLS
// mimicry profile reproduces. Chrome uses GREASE, compress_certificate, and
// ALPS extensions; Firefox uses NSS cipher ordering with padded ClientHello
// and delegated_credentials; Safari uses Apple's SecureTransport ordering with
// no GREASE and no padding. Only meaningful for ProfileTLS -- DNS/SIP/QUIC
// ignore it.
type BrowserProfile string

const (
	BrowserChrome  BrowserProfile = "chrome"
	BrowserFirefox BrowserProfile = "firefox"
	BrowserSafari  BrowserProfile = "safari"
)

// ObfProfile is the obfuscation strength preset. Lite = light junk + DNS I1;
// Standard = medium junk + TLS I1; Pro = heavy junk + full I1-I5;
// Premium = TLS 1.3 handshake sizes (S1/S2/S3) + H=1-4 under HPK + I1.
type ObfProfile string

const (
	ObfLite     ObfProfile = "lite"
	ObfStandard ObfProfile = "standard"
	ObfPro      ObfProfile = "pro"
	ObfPremium  ObfProfile = "premium"
)

// Domain pools. Ported from pumbaX/awg-multi-script (awg2.sh). These are
// well-known, globally-reachable front domains whose TLS/QUIC handshakes a
// censor is unlikely to block. The RU set adds Russian services for RU-based
// servers where those are the natural-looking traffic.

var tlsDomainsRU = []string{
	"google.com", "github.com", "microsoft.com", "apple.com",
	"cloudflare.com", "amazon.com", "youtube.com", "facebook.com",
	"instagram.com", "netflix.com", "spotify.com", "twitter.com",
	"linkedin.com", "yandex.ru", "vk.com", "mail.ru",
	"ozon.ru", "wildberries.ru", "rutube.ru", "gosuslugi.ru",
	"sberbank.ru", "tbank.ru",
}

var tlsDomainsWorld = []string{
	"google.com", "github.com", "microsoft.com", "apple.com",
	"cloudflare.com", "amazon.com", "youtube.com", "facebook.com",
	"instagram.com", "netflix.com", "spotify.com", "twitter.com",
	"linkedin.com", "wikipedia.org", "reddit.com", "dropbox.com",
	"digitalocean.com",
}

var sipDomains = []string{
	"sip.zadarma.com", "sip.iptel.org", "sip.linphone.org",
	"sip.dus.net", "sip.voys.nl", "sip.peoplefone.ch",
	"sip.messagenet.it", "sip.antisip.com", "sip.forumsip.com",
	"sip.telonline.com", "sip.1sip.co.uk",
}

var quicDomainsRU = []string{
	"www.google.com", "www.youtube.com", "cdn.jsdelivr.net",
	"unpkg.com", "icloud.com", "mzstatic.com", "www.fastly.com",
	"cdn.b-cdn.net", "github.com", "ozon.ru",
}

var quicDomainsWorld = []string{
	"www.google.com", "www.youtube.com", "cdn.jsdelivr.net",
	"unpkg.com", "icloud.com", "mzstatic.com", "www.fastly.com",
	"cdn.b-cdn.net", "github.com",
}

// dnsPool is the generic DNS-query front-domain set (no region split).
var dnsPool = []string{
	"google.com", "github.com", "microsoft.com", "apple.com",
	"cloudflare.com", "amazon.com", "youtube.com", "facebook.com",
	"instagram.com", "netflix.com", "wikipedia.org", "reddit.com",
	"dropbox.com", "digitalocean.com", "yandex.ru", "vk.com",
}

// DomainPool returns the domain list for the given profile and region.
func DomainPool(profile MimicryProfile, region Region) []string {
	switch profile {
	case ProfileTLS:
		if region == RegionRU {
			return tlsDomainsRU
		}
		return tlsDomainsWorld
	case ProfileQUIC:
		if region == RegionRU {
			return quicDomainsRU
		}
		return quicDomainsWorld
	case ProfileSIP:
		return sipDomains
	case ProfileDNS:
		return dnsPool
	default:
		return dnsPool
	}
}

// PickRandomDomain returns a random domain from the pool, or "" if the pool
// is empty. Uses math/rand — callers should ensure the package-level rand is
// seeded (the panel seeds it at startup).
func PickRandomDomain(pool []string) string {
	if len(pool) == 0 {
		return ""
	}
	return pool[rand.Intn(len(pool))]
}

// SelectDomain picks a front domain for the profile/region. If `domain` is
// non-empty it is returned verbatim (the caller asked to mimic a specific
// host); otherwise a random one is drawn from the pool.
func SelectDomain(profile MimicryProfile, region Region, domain string) string {
	if domain != "" {
		return domain
	}
	return PickRandomDomain(DomainPool(profile, region))
}
