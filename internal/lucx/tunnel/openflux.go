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
	Secret    string `json:"secret"`
	ShareHost string `json:"shareHost"`
	YandexURL string `json:"yandexUrl"`
	MailruURL string `json:"mailruUrl"`
	CupsURL   string `json:"cupsUrl"`
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
	return nil
}

func (c OpenfluxConfig) contextURL() string {
	for _, u := range []string{c.YandexURL, c.MailruURL, c.CupsURL} {
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
	writeDoc("yandex", 50, c.YandexURL)
	writeDoc("mailru", 40, c.MailruURL)
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
	add("yandex", 50, c.YandexURL)
	add("mailru", 40, c.MailruURL)
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
