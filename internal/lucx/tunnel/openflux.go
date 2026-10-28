// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"bytes"
	"compress/flate"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"
)

const (
	openfluxDefaultPort = 18445
	openfluxContext     = "http://openflux"
	openfluxMinSecret   = 16
	openfluxLinkPrefix  = "openflux://v1/"
)

// OpenfluxConfig is one OpenFlux exit (l4). Direct always listens on the
// inbound port. Document URLs are optional extra carriers. The exit serves
// one active client; the link is the credential.
type OpenfluxConfig struct {
	Secret      string `json:"secret"`
	ShareHost   string `json:"shareHost"`
	YandexURL   string `json:"yandexUrl"`
	MailruURL   string `json:"mailruUrl"`
	CupsURL     string `json:"cupsUrl"`
	VyandexURL  string `json:"vyandexUrl"`
	BoardsURL   string `json:"boardsUrl"`
	TelemostURL string `json:"telemostUrl"`
	AvitoURL    string `json:"avitoUrl"`
}

func DefaultOpenfluxConfig() OpenfluxConfig {
	return OpenfluxConfig{}
}

func (c OpenfluxConfig) Merge() OpenfluxConfig {
	c.Secret = strings.TrimSpace(c.Secret)
	c.ShareHost = strings.TrimSpace(c.ShareHost)
	c.YandexURL = strings.TrimSpace(c.YandexURL)
	c.MailruURL = strings.TrimSpace(c.MailruURL)
	c.CupsURL = strings.TrimSpace(c.CupsURL)
	c.VyandexURL = strings.TrimSpace(c.VyandexURL)
	c.BoardsURL = strings.TrimSpace(c.BoardsURL)
	c.TelemostURL = strings.TrimSpace(c.TelemostURL)
	c.AvitoURL = strings.TrimSpace(c.AvitoURL)
	return c
}

func (c OpenfluxConfig) EnsureSecret() (OpenfluxConfig, error) {
	if c.Secret != "" {
		return c, nil
	}
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return c, err
	}
	c.Secret = hex.EncodeToString(buf)
	return c, nil
}

func (c OpenfluxConfig) Validate(port int) error {
	if port < 1 || port > 65535 {
		return errors.New("openflux: port must be 1-65535")
	}
	if openfluxSecretChars(c.Secret) < openfluxMinSecret {
		return fmt.Errorf("openflux: secret must be at least %d characters", openfluxMinSecret)
	}
	if err := openfluxHostOK(c.ShareHost); err != nil {
		return err
	}
	for _, raw := range []string{c.YandexURL, c.MailruURL, c.CupsURL} {
		if err := openfluxDocURL(raw); err != nil {
			return err
		}
	}
	// LUCX-HOOK (issue #148): carriers the core allows in share links since
	// v0.13.0 — validated against the same link shapes the upstream node
	// wizard accepts (docs/boards/telemost URLs, avito room list).
	for _, raw := range []string{c.VyandexURL, c.BoardsURL, c.TelemostURL, c.AvitoURL} {
		if err := openfluxShareURL(raw); err != nil {
			return err
		}
	}
	return nil
}

func (c OpenfluxConfig) contextURL() string {
	for _, u := range []string{c.YandexURL, c.MailruURL, c.CupsURL, c.VyandexURL, c.BoardsURL, c.TelemostURL, c.AvitoURL} {
		if u != "" {
			return u
		}
	}
	return openfluxContext
}

func (c OpenfluxConfig) listenAddr(port int) string {
	return net.JoinHostPort("0.0.0.0", strconv.Itoa(port))
}

func (c OpenfluxConfig) dialAddr(host string, port int) string {
	host = strings.TrimSpace(host)
	if host == "" {
		host = c.ShareHost
	}
	if host == "" || port < 1 {
		return ""
	}
	return net.JoinHostPort(host, strconv.Itoa(port))
}

// RenderConf is the exit .conf. Mode is always l4 (no kernel RST drop).
// A [Transport] section makes the binary run a negotiated session.
func (c OpenfluxConfig) RenderConf(key string, port int) string {
	secretPath := absPath(filepath.Join(workDir(), key+"-secret.txt"))
	cookiePath := absPath(filepath.Join(dataDirFor(key, Openflux), "cookies.json"))
	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\nRole = exit\nMode = l4\nCodec = batched\nEncryptionKeyFile = %s\nURL = %s\nCookieStore = %s\n",
		secretPath, c.contextURL(), cookiePath)
	fmt.Fprintf(&b, "\n[Transport \"direct\"]\nType = direct\nPriority = 100\nListen = %s\n", c.listenAddr(port))
	writeDoc := func(name string, prio int, raw string) {
		if raw == "" {
			return
		}
		fmt.Fprintf(&b, "\n[Transport \"%s\"]\nType = %s\nPriority = %d\nURL = %s\n", name, name, prio, raw)
	}
	// LUCX-HOOK (issue #148): descending priority following the upstream
	// node wizard's relative carrier order (vyandex > yandex > mailru >
	// telemost > boards > avito > cupsonline) mapped under the direct-first
	// scheme the panel already ships. Existing three keep their priorities.
	writeDoc("vyandex", 52, c.VyandexURL)
	writeDoc("yandex", 50, c.YandexURL)
	writeDoc("mailru", 40, c.MailruURL)
	writeDoc("telemost", 35, c.TelemostURL)
	writeDoc("boards", 32, c.BoardsURL)
	writeDoc("avito", 31, c.AvitoURL)
	writeDoc("cupsonline", 30, c.CupsURL)
	return b.String()
}

func (c OpenfluxConfig) secretFileName(key string) string {
	return key + "-secret.txt"
}

// ClientURI is openflux://v1/ + raw DEFLATE + base64url. Empty when the
// public host is unknown (direct needs a dial address).
func (c OpenfluxConfig) ClientURI(name, host string, port int) (string, error) {
	dial := c.dialAddr(host, port)
	if dial == "" || c.Secret == "" {
		return "", nil
	}
	payload := openfluxShare{
		Name:       strings.TrimSpace(name),
		Negotiate:  true,
		Secret:     c.Secret,
		Context:    c.contextURL(),
		Transports: c.shareTransports(dial),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	w, err := flate.NewWriter(&buf, flate.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := w.Write(raw); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}
	return openfluxLinkPrefix + base64.RawURLEncoding.EncodeToString(buf.Bytes()), nil
}

func (c OpenfluxConfig) shareTransports(dial string) []openfluxShareTransport {
	out := []openfluxShareTransport{{
		Type:     "direct",
		Priority: 100,
		Dial:     dial,
	}}
	add := func(kind string, prio int, raw string) {
		if raw == "" {
			return
		}
		out = append(out, openfluxShareTransport{Type: kind, Priority: prio, URL: raw})
	}
	add("vyandex", 52, c.VyandexURL)
	add("yandex", 50, c.YandexURL)
	add("mailru", 40, c.MailruURL)
	add("telemost", 35, c.TelemostURL)
	add("boards", 32, c.BoardsURL)
	add("avito", 31, c.AvitoURL)
	add("cupsonline", 30, c.CupsURL)
	return out
}

type openfluxShare struct {
	Name       string                   `json:"name,omitempty"`
	Negotiate  bool                     `json:"negotiate,omitempty"`
	Secret     string                   `json:"secret,omitempty"`
	Context    string                   `json:"context,omitempty"`
	Transports []openfluxShareTransport `json:"transports"`
}

type openfluxShareTransport struct {
	Type     string `json:"type"`
	URL      string `json:"url,omitempty"`
	Priority int    `json:"priority,omitempty"`
	Dial     string `json:"dial,omitempty"`
}

func openfluxSecretChars(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func openfluxHostOK(host string) error {
	if host == "" {
		return nil
	}
	if strings.ContainsAny(host, " /#;?") || strings.Contains(host, "://") {
		return errors.New("openflux: shareHost must be a host or IP, not a URL")
	}
	return nil
}

func openfluxDocURL(raw string) error {
	if raw == "" {
		return nil
	}
	if strings.ContainsAny(raw, "#;\r\n") {
		return errors.New("openflux: document URL cannot contain # or ;")
	}
	u, err := url.Parse(raw)
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return errors.New("openflux: document URL must be http(s)")
	}
	return nil
}

// LUCX-HOOK (issue #148): link shapes mirrored from the upstream exit-node
// wizard (provision/transports.go) — same hosts and paths so an operator
// pastes the same link either tool accepts.
var (
	openfluxVolgaDocRe = regexp.MustCompile(`^https://(docs|disk)\.yandex\.(ru|com|by|kz|uz)/edit/d/[A-Za-z0-9_-]{16,200}$`)
	openfluxBoardsRe   = regexp.MustCompile(`^https://boards\.yandex\.ru/whiteboard/\?hash=[0-9a-f]{32}$`)
	openfluxTelemostRe = regexp.MustCompile(`^https://telemost\.yandex\.ru/j/[0-9]{6,32}$`)
	openfluxRoomListRe = regexp.MustCompile(`^[A-Za-z0-9_-]{8,}$`)
)

// openfluxShareURL validates the v0.13.0 shareable carriers: vyandex (Yandex
// volga doc), boards (whiteboard link), telemost (conference) and avito
// (packed room list, same shape as cupsonline).
func openfluxShareURL(raw string) error {
	if raw == "" {
		return nil
	}
	switch {
	case openfluxVolgaDocRe.MatchString(raw):
		return nil
	case openfluxBoardsRe.MatchString(raw):
		return nil
	case openfluxTelemostRe.MatchString(raw):
		return nil
	case openfluxRoomListRe.MatchString(raw) && len(raw) <= 4096:
		return nil
	}
	return errors.New("openflux: unsupported carrier URL — need a Yandex docs link, a whiteboard link, a Telemost j/… conference, or a room list token")
}
