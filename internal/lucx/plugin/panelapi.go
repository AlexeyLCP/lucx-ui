// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

/* Stage 3c: the panel API seam. A manifest may declare `panelApi` as a list
of "<resource>:<kind>" grants (kinds: read, write, *). The panel mints the
plugin a Bearer token at start (persisted as <id>/.panel-token, 0600) and
the plugin calls the panel API with Authorization: Bearer; the panel checks
every call against this manifest matrix plus a never-reachable route list.
Bot tokens (telegram/discord) reach the plugin as env vars when the
manifest's `bot` list names them; rotation happens on the next start.

Plugins granted panelApi or bot must NOT also declare network: ["none"] —
the kernel-isolated network namespace has no route to the panel (nor to the
bot APIs), and the panel refuses such manifests. */

const (
	// PanelTokenEnv carries the plugin's panel-API bearer token.
	PanelTokenEnv = "LUCX_PANEL_TOKEN"
	// PanelURLEnv carries the loopback base URL of the panel API.
	PanelURLEnv = "LUCX_PANEL_URL"
	// TelegramTokenEnv carries the configured Telegram bot token.
	TelegramTokenEnv = "LUCX_TELEGRAM_BOT_TOKEN"
	// DiscordTokenEnv carries the configured Discord bot token.
	DiscordTokenEnv = "LUCX_DISCORD_BOT_TOKEN"

	// panelTokenFile stores the plugin's panel credential next to its code.
	panelTokenFile = ".panel-token"
)

// panelResourceVocabulary fixes which first path segments under /panel/api a
// manifest may name. Mirrors the controller route groups; a web-side test
// keeps the list honest against the router.
var panelResourceVocabulary = map[string]bool{
	"clients":           true,
	"inbounds":          true,
	"hosts":             true,
	"server":            true,
	"awg-outbounds":     true,
	"sidecar-outbounds": true,
	"tunnel":            true,
}

// botVocabulary names the bots whose panel-stored tokens can be delivered.
var botVocabulary = map[string]bool{
	"telegram": true,
	"discord":  true,
}

/*
	neverReachableServer lists /server/... routes no plugin permission can

unlock: they hand over the database, rotate credentials or mutate the panel
itself. The list is checked before the resource matrix.
*/
var neverReachableServer = map[string]bool{
	"/server/importDB":                true,
	"/server/getDb":                   true,
	"/server/updatePanel":             true,
	"/server/getMigration":            true,
	"/server/restartPanel":            true,
	"/server/getNewXray":              true,
	"/server/getNewXrayGeodata":       true,
	"/server/getNewXrayCert":          true,
	"/server/getNewEchCert":           true,
	"/server/getNewPanelCert":         true,
	"/server/getNewMrzCert":           true,
	"/server/downloadXray":            true,
	"/server/checkPanelLoginSecurity": true,
	"/server/updateConfig":            true,
	"/server/restartXrayService":      true,
	"/server/stopXrayService":         true,
	"/server/restartXray":             true,
	"/server/installPanel":            true,
	"/server/logs/:type":              true,
}

func validatePanelPermissions(rules []string) error {
	if len(rules) > 16 {
		return errors.New("plugin permissions.panelApi has too many entries")
	}
	seen := make(map[string]bool, len(rules))
	for _, rule := range rules {
		res, kind, found := strings.Cut(rule, ":")
		if !found {
			return fmt.Errorf("plugin permission %q must be <resource>:read|write|*", rule)
		}
		if !panelResourceVocabulary[res] {
			return fmt.Errorf("unknown panelApi resource %q in apiVersion 1", res)
		}
		if kind != "read" && kind != "write" && kind != "*" {
			return fmt.Errorf("plugin permission %q has an unknown kind (read, write or *)", rule)
		}
		if seen[rule] {
			return fmt.Errorf("plugin permission %q is listed twice", rule)
		}
		seen[rule] = true
	}
	return nil
}

func validateBotPermissions(rules []string) error {
	if len(rules) > 16 {
		return errors.New("plugin permissions.bot has too many entries")
	}
	seen := make(map[string]bool, len(rules))
	for _, name := range rules {
		if !botVocabulary[name] {
			return fmt.Errorf("unknown bot %q in apiVersion 1 (telegram, discord)", name)
		}
		if seen[name] {
			return fmt.Errorf("bot %q is listed twice", name)
		}
		seen[name] = true
	}
	return nil
}

// panelDeniedByNetwork is the validated contract: an isolated plugin cannot
// talk to the panel or to a bot API.
func panelDeniedByNetwork(m Manifest) bool {
	return denyNetwork(m) && (len(m.Permissions["panelApi"]) > 0 || len(m.Permissions["bot"]) > 0)
}

var (
	panelTokenLock sync.Mutex
	panelTokens    = map[string]string{}
	panelURLLock   sync.Mutex
	panelURL       string
	botTokensLock  sync.Mutex
	botTokens      = map[string]string{}
)

// LoadPanelTokens re-arms the token matcher from disk. The panel calls it at
// startup before serving API routes; tokens persist across panel restarts so
// a restarted plugin keeps its granted credential.
func LoadPanelTokens() {
	panelTokenLock.Lock()
	defer panelTokenLock.Unlock()
	entries, err := os.ReadDir(root())
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir(e.Name()), panelTokenFile))
		if err != nil || len(b) < 32 {
			continue
		}
		panelTokens[e.Name()] = strings.TrimSpace(string(b))
	}
}

// GrantPanelToken returns (minting on first use) the plugin's panel-API
// token. It is written 0600 next to the plugin code; deleting the plugin
// drops it.
func GrantPanelToken(id string) (string, error) {
	if err := validateID(id); err != nil {
		return "", err
	}
	panelTokenLock.Lock()
	defer panelTokenLock.Unlock()
	if tok, ok := panelTokens[id]; ok {
		return tok, nil
	}
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	tok := hex.EncodeToString(raw)
	if err := os.MkdirAll(dir(id), 0o700); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(dir(id), panelTokenFile), []byte(tok), 0o600); err != nil {
		return "", err
	}
	panelTokens[id] = tok
	return tok, nil
}

func forgetPanelToken(id string) {
	panelTokenLock.Lock()
	defer panelTokenLock.Unlock()
	delete(panelTokens, id)
}

// MatchPanelToken resolves a presented bearer token to its plugin id.
func MatchPanelToken(token string) (string, bool) {
	if token == "" {
		return "", false
	}
	panelTokenLock.Lock()
	defer panelTokenLock.Unlock()
	for id, tok := range panelTokens {
		if tok == token {
			return id, true
		}
	}
	return "", false
}

// SetPanelURL tells the plugin runner where the panel API listens (loopback,
// without the /panel/api suffix). Called by the panel at startup.
func SetPanelURL(url string) {
	panelURLLock.Lock()
	panelURL = url
	panelURLLock.Unlock()
}

func currentPanelURL() string {
	panelURLLock.Lock()
	defer panelURLLock.Unlock()
	return panelURL
}

// SetBotTokens gives the runner the panel-stored bot credentials for env
// delivery; only names the manifest lists are handed over.
func SetBotTokens(tokens map[string]string) {
	botTokensLock.Lock()
	botTokens = map[string]string{}
	for k, v := range tokens {
		botTokens[k] = v
	}
	botTokensLock.Unlock()
}

func botTokenFor(name string) string {
	botTokensLock.Lock()
	defer botTokensLock.Unlock()
	return botTokens[name]
}

/*
	pluginEnv builds the extra env vars the seam grants for this manifest. It

returns the ordered KEY=VALUE list passed to both runners.
*/
func pluginEnv(m Manifest) []string {
	var env []string
	for _, name := range m.Permissions["bot"] {
		if tok := botTokenFor(name); tok != "" {
			switch name {
			case "telegram":
				env = append(env, TelegramTokenEnv+"="+tok)
			case "discord":
				env = append(env, DiscordTokenEnv+"="+tok)
			}
		}
	}
	if len(m.Permissions["panelApi"]) > 0 {
		env = append(env, PanelTokenEnv+"="+panelTokens[m.ID])
		if url := currentPanelURL(); url != "" {
			env = append(env, PanelURLEnv+"="+url)
		}
	}
	return env
}

/*
	PermisesPanelCall is the enforcement point the panel API middleware uses:

relPath is the gin route pattern relative to /panel/api. The
never-reachable list wins over everything; after that a call is allowed iff
the manifest grants the resource and the verb kind (GET/HEAD = read).
*/
func PluginPanelAllowed(id, relPath, method string) bool {
	if err := validateID(id); err != nil {
		return false
	}
	if neverReachableServer[relPath] {
		return false
	}
	if strings.HasPrefix(relPath, "/plugins") || strings.HasPrefix(relPath, "/setting") ||
		strings.HasPrefix(relPath, "/xray") || strings.HasPrefix(relPath, "/nodes") {
		return false
	}
	res := strings.TrimPrefix(strings.SplitN(relPath, "/", 3)[1], "/") // "/inbounds/list" -> "inbounds"
	rules := permissionRules(id, "panelApi")
	if len(rules) == 0 {
		return false
	}
	kind := "read"
	switch method {
	case http.MethodGet, http.MethodHead:
	default:
		kind = "write"
	}
	for _, rule := range rules {
		granted, k, found := strings.Cut(rule, ":")
		if !found {
			continue
		}
		if granted == res && (k == kind || k == "*") {
			return true
		}
	}
	return false
}

/*
	permissionRules reads the rules for a permission kind from the plugin's

installed manifest (panelApi / bot).
*/
func permissionRules(id, kind string) []string {
	m, err := Load(id)
	if err != nil {
		return nil
	}
	return m.Permissions[kind]
}
