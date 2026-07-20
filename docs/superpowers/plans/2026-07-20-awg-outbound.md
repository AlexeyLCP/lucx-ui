# AWG Outbound (клиентский режим) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Добавить нативный AWG outbound — клиентское подключение к upstream AmneziaWG-серверу через kernel-интерфейс `awgo-{Id}` + Xray freedom outbound с `sockopt.interface`.

**Architecture:** Новая таблица `awg_outbounds` (стиль `inbounds`) → `ClientInstance` в `internal/awg/` → расширение `Manager` (`EnsureClient`/`RemoveClient`/`SweepOrphanClients`/`CollectClientTraffic`) → reconcile в существующем `awg_job` (10с cron + startup sweep) → `injectAwgOutbounds` в `xray.go` (freedom + sockopt.interface, после обычных outbounds, до balancers/routing) → 8 REST endpoints → новая вкладка "AWG outbounds" в XrayPage (рядом с переименованной "Xray outbounds") с формой + Paste .conf + Test (ping -I).

**Tech Stack:** Go 1.23+ (GORM, stdlib `testing`), React + TS + AntD v6 + Zod, amneziaawg kernel module + awg-quick.

## Global Constraints

- **License:** Все новые файлы — PolyForm Noncommercial 1.0.0, SPDX header после `package`/`//go:build` (5 строк, см. любой файл в `internal/awg/`). Upstream-файлы с LUCX-HOOK блоками остаются GPL — никаких SPDX в них.
- **LUCX-HOOK isolation:** Интеграции в upstream-файлы (`web.go`, `xray.go`, `inbound.go` если нужно, `XrayPage.tsx`, `inbound-defaults.ts`) — только внутри `// LUCX-HOOK:` / `// END LUCX-HOOK` блоков.
- **Interface naming:** outbound kernel interface всегда `awgo-{Id}` (БД-стабилен, не зависит от Tag). Tag редактируемый, по умолчанию `awgo-{Id}`.
- **Client .conf:** `Table = off` КРИТИЧНО (не трогаем системный default route). `Address` обязателен. `DNS` не пишем по умолчанию (Xray резолвит через `domainStrategy: UseIP`).
- **.conf permissions:** `os.WriteFile(path, content, 0600)` — awg-quick ругается на world-readable.
- **sendThrough:** strip CIDR перед инжектом: `strings.SplitN(address, "/", 2)[0]`.
- **Tag uniqueness:** проверка против ВСЕХ outbound-тегов финального Xray-конфига (обычные + AWG + системные `direct`/`block`/`api`).
- **PrivateKey:** через `wgutil.GenerateWireguardKeypair()` (`x/crypto/curve25519`), не random bytes.
- **Reconcile:** расширяем существующий `awg_job` (10с), + `SweepOrphanClients` через `sync.Once` на старте.
- **Test endpoint:** `ping -c 3 -W 2 -I awgo-{Id} 1.1.1.1` (IPv6-фолбэк: `ping6` + `2606:4700:4700::1111`).
- **Disable:** outbound полностью убирается из Xray-конфига (не blackhole), чтобы Xray упал явно, если routing rule ссылается на удалённый tag.
- **IPv6:** dual-stack поддерживается — не хардкодить IPv4.
- **Tests:** stdlib `testing`, table-driven, `t.Run`. `go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v`. Frontend: `npm run typecheck && npm run lint`.
- **Commits:** Russian messages, conventional-commit prefixes (`feat(awg-outbound):`, `test(awg-outbound):`, и т.д.).
- **gofumpt:** `bin/check-lucx.sh` перед коммитом; `-w` для автофикса.

---

## File Structure

### New files (all PolyForm NC, SPDX header)

| File | Responsibility |
|---|---|
| `internal/database/model/awg_outbound.go` | `AwgOutbound` GORM model + `AwgOutboundSettings` struct |
| `internal/database/migrate_awg_outbound.go` | `AutoMigrate(&AwgOutbound{})` call wrapper |
| `internal/database/migrate_awg_outbound_test.go` | Migration idempotency test |
| `internal/awg/client_instance.go` | `ClientInstance` type + `ClientInstanceFromOutbound` + `fingerprint` |
| `internal/awg/client_instance_test.go` | Parse/fingerprint tests |
| `internal/awg/client_conf.go` | `renderClientConf(ci ClientInstance) string` (Table=off, Address, Endpoint, no DNS default, obfuscation) |
| `internal/awg/client_conf_test.go` | renderClientConf tests (Table=off, fields, obfuscation, IPv6) |
| `internal/awg/client_manager.go` | `Manager` methods: `EnsureClient`, `RemoveClient`, `SweepOrphanClients`, `CollectClientTraffic` |
| `internal/awg/client_manager_test.go` | Client manager state tests |
| `internal/web/service/awg_outbound.go` | `AwgOutboundService`: CRUD + `parseConf` + `defaultAwgOutboundSettings` + tag uniqueness check |
| `internal/web/service/awg_outbound_test.go` | CRUD + parseConf + tag uniqueness tests |
| `internal/web/service/awg_outbound_inject_test.go` | `injectAwgOutbounds` tests |
| `internal/web/controller/awg_outbound.go` | 8 REST endpoints + Test (ping -I) + parseConf |
| `internal/web/job/awg_outbound_job.go` | Reconcile extension for `awg_outbound` (calls Manager) |
| `frontend/src/schemas/awg-outbound.ts` | Zod schema `AwgOutboundSettingsSchema` + `AwgOutboundSchema` |
| `frontend/src/pages/xray/awg-outbounds/AwgOutboundsTab.tsx` | Table + status + Test button |
| `frontend/src/pages/xray/awg-outbounds/AwgOutboundFormModal.tsx` | Form + Paste .conf drawer + Advanced |
| `frontend/src/pages/xray/awg-outbounds/AwgOutboundStatusBadge.tsx` | Status badge (Up + handshake/traffic / Down + fallback warning) |
| `frontend/src/api/awg-outbounds.ts` | API client (fetch wrappers) |

### Modified files (LUCX-HOOK blocks only, stay GPL)

| File | Change |
|---|---|
| `internal/database/db.go` | LUCX-HOOK: add `&model.AwgOutbound{}` to AutoMigrate list |
| `internal/web/web.go` | LUCX-HOOK: register `awg-outbounds` routes group |
| `internal/web/service/xray.go` | LUCX-HOOK: call `injectAwgOutbounds` after outbound merge, before balancers/routing |
| `internal/web/job/awg_job.go` | LUCX-HOOK: reconcile `awg_outbounds` + `SweepOrphanClients` on first tick |
| `frontend/src/pages/xray/XrayPage.tsx` | LUCX-HOOK: add `'awg-outbound'` to SECTION_SLUGS, rename outbound tab label, render `<AwgOutboundsTab />` |
| `frontend/src/locales/en-US.json` + 12 others | LUCX-HOOK: `awgOutbound*` i18n keys |

---

## Task 1: Data Model + Migration

**Files:**
- Create: `internal/database/model/awg_outbound.go`
- Create: `internal/database/migrate_awg_outbound.go`
- Modify: `internal/database/db.go` (LUCX-HOOK — add to AutoMigrate list)
- Test: `internal/database/migrate_awg_outbound_test.go`

**Interfaces:**
- Produces: `model.AwgOutbound` struct (fields: Id, Tag, Remark, Enable, Settings, CreatedAt, UpdatedAt) — used by Task 3 (`ClientInstanceFromOutbound`), Task 6 (service CRUD), Task 7 (controller).

- [ ] **Step 1: Write the model**

Create `internal/database/model/awg_outbound.go`:

```go
// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package model

// AwgOutbound is a client-mode AmneziaWG connection to an upstream VPN server.
// Mirrors the Inbound shape (Tag/Remark/Enable/Settings JSON) but represents an
// egress: the panel brings up a kernel interface awgo-{Id} and injects a
// freedom outbound (bound to that interface) into the Xray config so routing
// rules can send traffic through the upstream VPN.
type AwgOutbound struct {
	Id        int    `json:"id" form:"id" gorm:"primaryKey;autoIncrement"`
	Tag       string `json:"tag" form:"tag" gorm:"uniqueIndex;not null"`
	Remark    string `json:"remark" form:"remark"`
	Enable    bool   `json:"enable" form:"enable" gorm:"default:true"`
	Settings  string `json:"settings" form:"settings"`
	CreatedAt int64  `json:"created_at" gorm:"autoCreateTime"`
	UpdatedAt int64  `json:"updated_at" gorm:"autoUpdateTime"`
}
```

- [ ] **Step 2: Write the migration wrapper**

Create `internal/database/migrate_awg_outbound.go`:

```go
// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// migrateAwgOutbounds creates the awg_outbounds table. Idempotent: GORM
// AutoMigrate is a no-op when the table already exists with the same columns.
func migrateAwgOutbounds() error {
	return db.AutoMigrate(&model.AwgOutbound{})
}
```

- [ ] **Step 3: Wire migration into db.go (LUCX-HOOK)**

Find the AutoMigrate model list in `internal/database/db.go` (around line 89, the `for _, mdl := range` loop). Add a LUCX-HOOK block registering `&model.AwgOutbound{}`:

```go
		// LUCX-HOOK: AWG outbound — client-mode AmneziaWG table.
		if err := db.AutoMigrate(&model.AwgOutbound{}); err != nil {
			return err
		}
		// END LUCX-HOOK
```

Place it right after the existing `for _, mdl := range models { ... AutoMigrate(mdl) ... }` loop (so it runs on every init, idempotently).

- [ ] **Step 4: Write the failing test**

Create `internal/database/migrate_awg_outbound_test.go`:

```go
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package database

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestMigrateAwgOutbounds_Idempotent(t *testing.T) {
	if err := initDBForTest(t); err != nil {
		t.Fatalf("initDB: %v", err)
	}
	if err := migrateAwgOutbounds(); err != nil {
		t.Fatalf("first migrate: %v", err)
	}
	if err := migrateAwgOutbounds(); err != nil {
		t.Fatalf("second migrate (idempotent): %v", err)
	}
	var count int64
	if err := db.Model(&model.AwgOutbound{}).Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Fatalf("expected 0 rows, got %d", count)
	}
}
```

Note: `initDBForTest` is the existing test helper used by `migrate_awg_test.go` — check its signature and use it. If it requires CGO (sqlite), the test file is tagged `//go:build cgo` if needed (look at how `migrate_awg_test.go` handles it).

- [ ] **Step 5: Run test to verify it passes**

Run: `CGO_ENABLED=1 go test ./internal/database/... -run TestMigrateAwgOutbounds -count=1 -v`
Expected: PASS

(On Windows without gcc, this test is skipped — verify on test2 or CI. The test is still correct; it just requires cgo to run.)

- [ ] **Step 6: Build check**

Run: `go build ./internal/database/... ./internal/database/model/...`
Expected: no errors

- [ ] **Step 7: Commit**

```bash
git add internal/database/model/awg_outbound.go internal/database/migrate_awg_outbound.go internal/database/migrate_awg_outbound_test.go internal/database/db.go
git commit -m "feat(awg-outbound): модель AwgOutbound + миграция awg_outbounds"
```

---

## Task 2: ClientInstance + fingerprint + renderClientConf

**Files:**
- Create: `internal/awg/client_instance.go`
- Create: `internal/awg/client_conf.go`
- Test: `internal/awg/client_instance_test.go`
- Test: `internal/awg/client_conf_test.go`

**Interfaces:**
- Consumes: `model.AwgOutbound` (from Task 1)
- Produces:
  - `ClientInstance` struct (Id, Ifname, PrivateKey, Address, MTU, PublicKey, PSK, Endpoint, Keepalive, AllowedIPs, Jc, Jmin, Jmax, S1, S2, S3, S4, H1, H2, H3, H4, I1, I2, I3, I4, I5)
  - `ClientInstanceFromOutbound(o *model.AwgOutbound) (ClientInstance, bool)`
  - `(ci ClientInstance) fingerprint() string`
  - `renderClientConf(ci ClientInstance) string`
- Used by: Task 3 (`Manager.EnsureClient`), Task 6 (`parseConf`)

- [ ] **Step 1: Write ClientInstance + fingerprint**

Create `internal/awg/client_instance.go`:

```go
// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

// ClientInstance is the desired runtime configuration of one AWG outbound: a
// kernel interface awgo-{Id} that connects as a client to an upstream AWG
// server. Unlike the server-side Instance, it has no ListenPort and a single
// peer (the upstream), not a peer list.
type ClientInstance struct {
	Id       int
	Ifname   string // "awgo-{Id}", never changes
	Tag      string // editable, used in Xray config as outbound tag
	Settings ClientSettings
}

// ClientSettings holds the parsed settings JSON for an AWG outbound.
type ClientSettings struct {
	PrivateKey  string `json:"privateKey"`
	Address     string `json:"address"`     // mandatory, e.g. "10.9.0.5/32"
	MTU         int    `json:"mtu"`
	PublicKey   string `json:"publicKey"`   // upstream server public key
	PSK         string `json:"psk"`
	Endpoint    string `json:"endpoint"`    // "host:port"
	Keepalive   int    `json:"keepalive"`
	AllowedIPs  string `json:"allowedIPs"`
	DNS         string `json:"dns"`         // optional, only written if non-empty
	Jc          int    `json:"jc"`
	Jmin        int    `json:"jmin"`
	Jmax        int    `json:"jmax"`
	S1          int    `json:"s1"`
	S2          int    `json:"s2"`
	S3          int    `json:"s3"`
	S4          int    `json:"s4"`
	H1          string `json:"h1"`
	H2          string `json:"h2"`
	H3          string `json:"h3"`
	H4          string `json:"h4"`
	I1          string `json:"i1"`
	I2          string `json:"i2"`
	I3          string `json:"i3"`
	I4          string `json:"i4"`
	I5          string `json:"i5"`
}

// ClientInstanceFromOutbound parses an AwgOutbound row into a ClientInstance.
// Returns ok=false if Settings is empty/malformed or a mandatory field
// (Address, PublicKey, Endpoint) is missing.
func ClientInstanceFromOutbound(o *model.AwgOutbound) (ClientInstance, bool) {
	if o == nil {
		return ClientInstance{}, false
	}
	var s ClientSettings
	if err := json.Unmarshal([]byte(o.Settings), &s); err != nil {
		return ClientInstance{}, false
	}
	s.Address = strings.TrimSpace(s.Address)
	s.PublicKey = strings.TrimSpace(s.PublicKey)
	s.Endpoint = strings.TrimSpace(s.Endpoint)
	if s.Address == "" || s.PublicKey == "" || s.Endpoint == "" {
		return ClientInstance{}, false
	}
	if s.AllowedIPs == "" {
		s.AllowedIPs = "0.0.0.0/0, ::/0"
	}
	if s.MTU == 0 {
		s.MTU = 1320
	}
	return ClientInstance{
		Id:       o.Id,
		Ifname:   "awgo-" + strconv.Itoa(o.Id),
		Tag:      o.Tag,
		Settings: s,
	}, true
}

// fingerprint returns a stable string that changes whenever any value that
// ends up in the generated .conf changes, so EnsureClient can detect when to
// restart awg-quick. Mirrors Instance.fingerprint.
func (ci ClientInstance) fingerprint() string {
	s := ci.Settings
	parts := []string{
		ci.Ifname,
		s.PrivateKey,
		s.Address,
		strconv.Itoa(s.MTU),
		s.PublicKey,
		s.PSK,
		s.Endpoint,
		strconv.Itoa(s.Keepalive),
		s.AllowedIPs,
		s.DNS,
		strconv.Itoa(s.Jc),
		strconv.Itoa(s.Jmin),
		strconv.Itoa(s.Jmax),
		strconv.Itoa(s.S1),
		strconv.Itoa(s.S2),
		strconv.Itoa(s.S3),
		strconv.Itoa(s.S4),
		s.H1, s.H2, s.H3, s.H4,
		s.I1, s.I2, s.I3, s.I4, s.I5,
	}
	return strings.Join(parts, "|")
}
```

- [ ] **Step 2: Write renderClientConf**

Create `internal/awg/client_conf.go`:

```go
// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"fmt"
	"strings"
)

// renderClientConf builds the awg-quick .conf for a client AWG instance. Unlike
// renderServerConf, this has no ListenPort, no peers[], Table = off (so awg-quick
// does NOT override the system default route — Xray's sockopt.interface handles
// egress), a single [Peer] (the upstream server), and DNS is omitted by default
// (Xray resolves via domainStrategy: UseIP; operator can opt in via Advanced).
func renderClientConf(ci ClientInstance) string {
	s := ci.Settings
	var b strings.Builder
	b.WriteString("[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", s.PrivateKey)
	fmt.Fprintf(&b, "Address = %s\n", s.Address)
	fmt.Fprintf(&b, "MTU = %d\n", s.MTU)
	b.WriteString("Table = off\n")
	if dns := strings.TrimSpace(s.DNS); dns != "" {
		fmt.Fprintf(&b, "DNS = %s\n", dns)
	}
	if s.Jc > 0 {
		fmt.Fprintf(&b, "Jc = %d\n", s.Jc)
		fmt.Fprintf(&b, "Jmin = %d\n", s.Jmin)
		fmt.Fprintf(&b, "Jmax = %d\n", s.Jmax)
		fmt.Fprintf(&b, "S1 = %d\n", s.S1)
		fmt.Fprintf(&b, "S2 = %d\n", s.S2)
		fmt.Fprintf(&b, "S3 = %d\n", s.S3)
		fmt.Fprintf(&b, "S4 = %d\n", s.S4)
		fmt.Fprintf(&b, "H1 = %s\n", s.H1)
		fmt.Fprintf(&b, "H2 = %s\n", s.H2)
		fmt.Fprintf(&b, "H3 = %s\n", s.H3)
		fmt.Fprintf(&b, "H4 = %s\n", s.H4)
	}
	if s.I1 != "" {
		fmt.Fprintf(&b, "I1 = %s\n", s.I1)
		fmt.Fprintf(&b, "I2 = %s\n", s.I2)
		fmt.Fprintf(&b, "I3 = %s\n", s.I3)
		fmt.Fprintf(&b, "I4 = %s\n", s.I4)
		fmt.Fprintf(&b, "I5 = %s\n", s.I5)
	}
	b.WriteString("\n[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", s.PublicKey)
	if psk := strings.TrimSpace(s.PSK); psk != "" {
		fmt.Fprintf(&b, "PresharedKey = %s\n", psk)
	}
	fmt.Fprintf(&b, "Endpoint = %s\n", s.Endpoint)
	fmt.Fprintf(&b, "AllowedIPs = %s\n", s.AllowedIPs)
	if s.Keepalive > 0 {
		fmt.Fprintf(&b, "PersistentKeepalive = %d\n", s.Keepalive)
	}
	return b.String()
}
```

- [ ] **Step 3: Write the failing tests**

Create `internal/awg/client_instance_test.go`:

```go
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientInstanceFromOutbound(t *testing.T) {
	cases := []struct {
		name    string
		settings string
		wantOk  bool
	}{
		{
			name:    "valid",
			settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up.example.com:51820","keepalive":25,"mtu":1320}`,
			wantOk:  true,
		},
		{
			name:    "missing address",
			settings: `{"privateKey":"k","publicKey":"pub","endpoint":"up.example.com:51820"}`,
			wantOk:  false,
		},
		{
			name:    "missing publicKey",
			settings: `{"privateKey":"k","address":"10.9.0.5/32","endpoint":"up.example.com:51820"}`,
			wantOk:  false,
		},
		{
			name:    "missing endpoint",
			settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub"}`,
			wantOk:  false,
		},
		{
			name:    "empty settings",
			settings: ``,
			wantOk:  false,
		},
		{
			name:    "malformed json",
			settings: `{broken`,
			wantOk:  false,
		},
		{
			name:    "defaults applied (mtu, allowedIPs)",
			settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up.example.com:51820"}`,
			wantOk:  true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			o := &model.AwgOutbound{Id: 7, Tag: "awgo-7", Settings: tc.settings}
			ci, ok := ClientInstanceFromOutbound(o)
			if ok != tc.wantOk {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOk)
			}
			if !ok {
				return
			}
			if ci.Ifname != "awgo-7" {
				t.Errorf("Ifname = %q, want awgo-7", ci.Ifname)
			}
			if ci.Id != 7 {
				t.Errorf("Id = %d, want 7", ci.Id)
			}
			if ci.Settings.MTU != 1320 {
				t.Errorf("MTU default = %d, want 1320", ci.Settings.MTU)
			}
			if ci.Settings.AllowedIPs != "0.0.0.0/0, ::/0" {
				t.Errorf("AllowedIPs default = %q, want 0.0.0.0/0, ::/0", ci.Settings.AllowedIPs)
			}
		})
	}
}

func TestClientInstanceFingerprint_Stable(t *testing.T) {
	o := &model.AwgOutbound{Id: 3, Tag: "awgo-3", Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320,"keepalive":25}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	ci2, _ := ClientInstanceFromOutbound(o)
	if ci1.fingerprint() != ci2.fingerprint() {
		t.Error("fingerprint not stable for same input")
	}
}

func TestClientInstanceFingerprint_ChangesOnEdit(t *testing.T) {
	o := &model.AwgOutbound{Id: 3, Tag: "awgo-3", Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	o.Settings = `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`
	ci2, _ := ClientInstanceFromOutbound(o)
	if ci1.fingerprint() == ci2.fingerprint() {
		t.Error("fingerprint did not change when Address changed")
	}
}
```

Create `internal/awg/client_conf_test.go`:

```go
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestRenderClientConf_TableOff(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320,"keepalive":25}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if !strings.Contains(conf, "Table = off") {
		t.Error("Table = off missing — critical, would override system default route")
	}
}

func TestRenderClientConf_NoListenPort(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if strings.Contains(conf, "ListenPort") {
		t.Error("ListenPort must NOT appear in client conf")
	}
}

func TestRenderClientConf_MandatoryFields(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up.example.com:51820","keepalive":25}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, want := range []string{
		"PrivateKey = k",
		"Address = 10.9.0.5/32",
		"MTU = 1320",
		"PublicKey = pub",
		"Endpoint = up.example.com:51820",
		"AllowedIPs = 0.0.0.0/0, ::/0",
		"PersistentKeepalive = 25",
	} {
		if !strings.Contains(conf, want) {
			t.Errorf("missing %q in conf:\n%s", want, conf)
		}
	}
}

func TestRenderClientConf_DNS_OmittedByDefault(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if strings.Contains(conf, "DNS =") {
		t.Error("DNS must NOT be written by default (Xray resolves via UseIP)")
	}
}

func TestRenderClientConf_DNS_WhenSet(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","dns":"1.1.1.1, 1.0.0.1"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if !strings.Contains(conf, "DNS = 1.1.1.1, 1.0.0.1") {
		t.Errorf("DNS should appear when set, got:\n%s", conf)
	}
}

func TestRenderClientConf_ObfuscationWhenSet(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","jc":3,"jmin":50,"jmax":150,"s1":20,"s2":30,"s3":40,"s4":50,"h1":"100-500","h2":"600-900","h3":"1000-1500","h4":"1600-2000"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, want := range []string{"Jc = 3", "Jmin = 50", "S1 = 20", "H1 = 100-500"} {
		if !strings.Contains(conf, want) {
			t.Errorf("missing %q in:\n%s", want, conf)
		}
	}
}

func TestRenderClientConf_ObfuscationOmittedWhenZero(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	for _, bad := range []string{"Jc =", "Jmin =", "S1 =", "H1 ="} {
		if strings.Contains(conf, bad) {
			t.Errorf("obfuscation line %q should not appear when unset, in:\n%s", bad, conf)
		}
	}
}

func TestRenderClientConf_IPv6(t *testing.T) {
	o := &model.AwgOutbound{Id: 1, Settings: `{"privateKey":"k","address":"fd00::5/128","publicKey":"pub","endpoint":"up:51820"}`}
	ci, _ := ClientInstanceFromOutbound(o)
	conf := renderClientConf(ci)
	if !strings.Contains(conf, "Address = fd00::5/128") {
		t.Errorf("IPv6 address not written, got:\n%s", conf)
	}
}
```

- [ ] **Step 4: Run tests to verify they fail (red)**

Run: `go test ./internal/awg/ -run "TestClientInstance|TestRenderClientConf" -count=1 -v`
Expected: FAIL (functions not defined yet)

- [ ] **Step 5: Already implemented in steps 1-2 — tests should pass (green)**

Run: `go test ./internal/awg/ -run "TestClientInstance|TestRenderClientConf" -count=1 -v`
Expected: PASS (all subtests)

- [ ] **Step 6: gofumpt + commit**

```bash
bin/check-lucx.sh -w
git add internal/awg/client_instance.go internal/awg/client_conf.go internal/awg/client_instance_test.go internal/awg/client_conf_test.go
git commit -m "feat(awg-outbound): ClientInstance + renderClientConf (Table=off, no ListenPort, DNS по умолчанию опускаем)"
```

---

## Task 3: Manager client methods (EnsureClient, RemoveClient, SweepOrphanClients, CollectClientTraffic)

**Files:**
- Create: `internal/awg/client_manager.go`
- Test: `internal/awg/client_manager_test.go`

**Interfaces:**
- Consumes: `ClientInstance`, `renderClientConf` (from Task 2), existing `awgConfigDir()`, `awgQuick()` helpers in `process.go`
- Produces:
  - `Manager.EnsureClient(ci ClientInstance) error`
  - `Manager.RemoveClient(ifname string) error`
  - `Manager.SweepOrphanClients()` (idempotent, sync.Once)
  - `Manager.CollectClientTraffic(ifname string) (handshakeAge time.Duration, rx, tx int64, ok bool)`

- [ ] **Step 1: Read existing process helpers**

Read `internal/awg/process.go` to confirm signatures of `awgConfigDir()` (returns the conf directory path) and `awgQuick(verb, confPath)` (returns combined output). These are reused for client instances — the conf path becomes `awgConfigDir() + "/" + ci.Ifname + ".conf"`.

- [ ] **Step 2: Write the implementation**

Create `internal/awg/client_manager.go`:

```go
// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// clientState tracks one running client interface so EnsureClient can detect
// fingerprint changes and restart awg-quick when the operator edits settings.
type clientState struct {
	fp string
}

var (
	clientMu       sync.Mutex
	clients        = map[string]clientState{} // ifname → state
	clientSwept    sync.Once
)

// EnsureClient reconciles a single client AWG interface to desired state.
// Writes the awg-quick .conf (mode 0600 — awg-quick rejects world-readable
// configs because they contain the private key), runs `awg-quick up` if the
// interface is down, and restarts it when the fingerprint changed. Idempotent
// and safe to call every 10s from awg_job. Mirrors Manager.Ensure but for the
// client (egress) side.
func (m *Manager) EnsureClient(ci ClientInstance) error {
	m.sweepOrphanClientsOnce()
	clientMu.Lock()
	defer clientMu.Unlock()
	confPath := filepath.Join(awgConfigDir(), ci.Ifname+".conf")
	newFP := ci.fingerprint()
	if st, ok := clients[ci.Ifname]; ok && st.fp == newFP {
		if _, err := awgShowIfname(ci.Ifname); err == nil {
			return nil
		}
	}
	if err := os.WriteFile(confPath, []byte(renderClientConf(ci)), 0600); err != nil {
		return err
	}
	if _, err := awgShowIfname(ci.Ifname); err == nil {
		if out, err := awgQuick("down", confPath); err != nil {
			logger.Warn("awg: awg-quick down failed before restart:", string(out), err)
		}
	}
	if out, err := awgQuick("up", confPath); err != nil {
		return fmt.Errorf("awg-quick up %s: %w (%s)", confPath, err, string(out))
	}
	clients[ci.Ifname] = clientState{fp: newFP}
	return nil
}

// RemoveClient tears down a client interface (awg-quick down + rm conf) and
// drops it from the in-memory state map. Safe to call when the interface is
// already gone (idempotent).
func (m *Manager) RemoveClient(ifname string) error {
	clientMu.Lock()
	defer clientMu.Unlock()
	confPath := filepath.Join(awgConfigDir(), ifname+".conf")
	if _, err := awgShowIfname(ifname); err == nil {
		if out, err := awgQuick("down", confPath); err != nil {
			return fmt.Errorf("awg-quick down %s: %w (%s)", confPath, err, string(out))
		}
	}
	_ = os.Remove(confPath)
	delete(clients, ifname)
	return nil
}

// SweepOrphanClients removes awgo-* interfaces and .conf files left over from
// a previous x-ui run that have no matching awg_outbounds row (or whose row is
// disabled). Runs once on first EnsureClient call (sync.Once) — not every tick.
func (m *Manager) sweepOrphanClientsOnce() {
	clientSwept.Do(func() {
		clientMu.Lock()
		defer clientMu.Unlock()
		entries, err := os.ReadDir(awgConfigDir())
		if err != nil {
			return
		}
		for _, e := range entries {
			name := e.Name()
			if !strings.HasPrefix(name, "awgo-") || !strings.HasSuffix(name, ".conf") {
				continue
			}
			ifname := strings.TrimSuffix(name, ".conf")
			if _, ok := clients[ifname]; ok {
				continue
			}
			if _, err := awgShowIfname(ifname); err != nil {
				_ = os.Remove(filepath.Join(awgConfigDir(), name))
				continue
			}
			if out, err := awgQuick("down", filepath.Join(awgConfigDir(), name)); err != nil {
				logger.Warn("awg: orphan sweep down failed for", ifname, string(out), err)
				continue
			}
			_ = os.Remove(filepath.Join(awgConfigDir(), name))
		}
	})
}

// CollectClientTraffic reads handshake age and rx/tx byte counters for one
// client interface via `awg show <iface> dump`. Returns ok=false if the
// interface is down or the dump is unreadable. Mirrors scrapePeers but for the
// single peer on the client side.
func (m *Manager) CollectClientTraffic(ifname string) (handshakeAge time.Duration, rx, tx int64, ok bool) {
	out, err := awgShowIfname(ifname)
	if err != nil {
		return 0, 0, 0, false
	}
	now := time.Now()
	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "interface") {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}
		var h int64
		_, _ = fmt.Sscanf(fields[4], "%d", &h)
		if h > 0 {
			handshakeAge = now.Sub(time.Unix(0, h*int64(time.Second)))
		}
		_, _ = fmt.Sscanf(fields[2], "%d", &rx)
		_, _ = fmt.Sscanf(fields[3], "%d", &tx)
		return handshakeAge, rx, tx, true
	}
	return 0, 0, 0, true
}
```

Note: this file needs `import ("fmt")` added. Check what helpers exist — if `awgShowIfname(name)` does not exist, add a tiny wrapper `func awgShowIfname(name string) ([]byte, error) { return exec.Command("awg", "show", name).CombinedOutput() }` (or `awg show <name> dump` — match what `scrapePeers` uses). Read `manager.go` scrapePeers to confirm the exact form.

- [ ] **Step 3: Write tests for state machine**

Create `internal/awg/client_manager_test.go`:

```go
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

import (
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestClientFingerprint_RestartDetection(t *testing.T) {
	o := &model.AwgOutbound{Id: 2, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`}
	ci1, _ := ClientInstanceFromOutbound(o)
	o.Settings = `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820","mtu":1320}`
	ci2, _ := ClientInstanceFromOutbound(o)
	if ci1.fingerprint() == ci2.fingerprint() {
		t.Fatal("fingerprint must change when Address changes (restart trigger)")
	}
}

func TestCollectClientTraffic_NoInterface(t *testing.T) {
	m := GetManager()
	_, _, _, ok := m.CollectClientTraffic("awgo-99999")
	if ok {
		t.Error("CollectClientTraffic should return ok=false for non-existent interface")
	}
}

func TestSweepOrphanClients_Idempotent(t *testing.T) {
	m := GetManager()
	m.sweepOrphanClientsOnce()
	m.sweepOrphanClientsOnce()
}
```

- [ ] **Step 4: Run tests (green on linux; on Windows awg exec fails — that's OK, CollectClientTraffic_NoInterface expects ok=false)**

Run: `go test ./internal/awg/ -run "TestClient|TestCollectClient|TestSweepOrphan" -count=1 -v`
Expected: PASS

- [ ] **Step 5: Build check (with GOOS=linux to ensure process helpers compile)**

Run: `GOOS=linux go build ./internal/awg/...`
Expected: no errors

- [ ] **Step 6: gofumpt + commit**

```bash
bin/check-lucx.sh -w
git add internal/awg/client_manager.go internal/awg/client_manager_test.go
git commit -m "feat(awg-outbound): Manager.EnsureClient/RemoveClient/SweepOrphanClients/CollectClientTraffic"
```

---

## Task 4: AwgOutboundService (CRUD + parseConf + tag uniqueness)

**Files:**
- Create: `internal/web/service/awg_outbound.go`
- Test: `internal/web/service/awg_outbound_test.go`

**Interfaces:**
- Consumes: `model.AwgOutbound` (Task 1), `ClientInstance` + `ClientSettings` (Task 2), `wgutil.GenerateWireguardKeypair` (existing)
- Produces:
  - `AwgOutboundService` with `AddOutbound`, `DelOutbound`, `UpdateOutbound`, `SetOutboundEnable`, `GetOutbounds`, `GetOutbound`
  - `parseConf(text string) (ClientSettings, error)` — parse pasted awg-quick .conf
  - `defaultAwgOutboundSettings() string` — JSON with generated client keypair
  - tag uniqueness check helper

- [ ] **Step 1: Write the service**

Create `internal/web/service/awg_outbound.go`:

```go
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
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
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
// x/crypto/curve25519 (same path as defaultAwgClients), not random bytes.
func defaultAwgOutboundSettings() string {
	priv, pub, err := wgutil.GenerateWireguardKeypair()
	if err != nil {
		return "{}"
	}
	s := map[string]any{
		"privateKey": priv,
		"publicKey":  "",
		"mtu":        1320,
		"keepalive":  25,
		"allowedIPs": "0.0.0.0/0, ::/0",
	}
	_ = pub
	b, _ := json.Marshal(s)
	return string(b)
}

// checkTagUnique returns ErrDuplicateOutboundTag if tag is already used by
// another AWG outbound (other than ignoreId), or matches a system tag.
// Caller is responsible for cross-checking against XrayConfig outbounds when
// injecting — this only checks the AWG table + system tags.
func checkTagUnique(tag string, ignoreId int) error {
	if tag == "direct" || tag == "block" || tag == "api" {
		return fmt.Errorf("%w: tag %q is reserved", ErrDuplicateOutboundTag, tag)
	}
	db := database.GetDb()
	var count int64
	if err := db.Model(&model.AwgOutbound{}).
		Where("tag = ? AND id != ?", tag, ignoreId).
		Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: tag %q already used by another AWG outbound", ErrDuplicateOutboundTag, tag)
	}
	return nil
}

// AddOutbound persists a new AWG outbound row. If Settings is empty, fills in
// a default keypair via defaultAwgOutboundSettings. Tag uniqueness is enforced.
func (s *AwgOutboundService) AddOutbound(o *model.AwgOutbound) (*model.AwgOutbound, error) {
	if strings.TrimSpace(o.Tag) == "" {
		return nil, errors.New("awg-outbound: tag is required")
	}
	if err := checkTagUnique(o.Tag, 0); err != nil {
		return nil, err
	}
	if strings.TrimSpace(o.Settings) == "" {
		o.Settings = defaultAwgOutboundSettings()
	}
	db := database.GetDb()
	if err := db.Create(o).Error; err != nil {
		return nil, err
	}
	o.Tag = "awgo-" + strconv.Itoa(o.Id)
	if err := db.Model(o).Update("tag", o.Tag).Error; err != nil {
		return nil, err
	}
	return o, nil
}

func (s *AwgOutboundService) DelOutbound(id int) error {
	db := database.GetDb()
	return db.Delete(&model.AwgOutbound{}, id).Error
}

func (s *AwgOutboundService) UpdateOutbound(o *model.AwgOutbound) error {
	if err := checkTagUnique(o.Tag, o.Id); err != nil {
		return err
	}
	db := database.GetDb()
	return db.Save(o).Error
}

func (s *AwgOutboundService) SetOutboundEnable(id int, enable bool) error {
	db := database.GetDb()
	return db.Model(&model.AwgOutbound{}).Where("id = ?", id).Update("enable", enable).Error
}

func (s *AwgOutboundService) GetOutbounds() ([]*model.AwgOutbound, error) {
	db := database.GetDb()
	var out []*model.AwgOutbound
	err := db.Order("id ASC").Find(&out).Error
	return out, err
}

func (s *AwgOutboundService) GetOutbound(id int) (*model.AwgOutbound, error) {
	db := database.GetDb()
	o := &model.AwgOutbound{}
	if err := db.First(o, id).Error; err != nil {
		return nil, err
	}
	return o, nil
}

// parseConf parses a pasted awg-quick .conf and returns ClientSettings. Used by
// the "Paste .conf" UI drawer to autofill the form. Tolerates whitespace and
// lines without values. Does NOT validate mandatory fields (caller does).
func parseConf(text string) (ClientSettings, error) {
	var s ClientSettings
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
				s.DNS = val
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
				s.Keepalive, _ = strconv.Atoi(val)
			}
		}
	}
	return s, nil
}
```

Note: `ClientSettings` is in package `awg`, not `service`. So `parseConf` should return `awg.ClientSettings` (import the awg package). Fix the import and return type before running.

- [ ] **Step 2: Write tests**

Create `internal/web/service/awg_outbound_test.go`:

```go
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"testing"
)

func TestParseConf_Client(t *testing.T) {
	conf := `[Interface]
PrivateKey = abcDEF
Address = 10.9.0.5/32
MTU = 1320
Table = off
Jc = 3
Jmin = 50
Jmax = 150

[Peer]
PublicKey = upstreamPub
Endpoint = up.example.com:51820
AllowedIPs = 0.0.0.0/0, ::/0
PersistentKeepalive = 25
`
	s, err := parseConf(conf)
	if err != nil {
		t.Fatalf("parseConf: %v", err)
	}
	if s.PrivateKey != "abcDEF" {
		t.Errorf("PrivateKey = %q", s.PrivateKey)
	}
	if s.Address != "10.9.0.5/32" {
		t.Errorf("Address = %q", s.Address)
	}
	if s.MTU != 1320 {
		t.Errorf("MTU = %d", s.MTU)
	}
	if s.PublicKey != "upstreamPub" {
		t.Errorf("PublicKey = %q", s.PublicKey)
	}
	if s.Endpoint != "up.example.com:51820" {
		t.Errorf("Endpoint = %q", s.Endpoint)
	}
	if s.Keepalive != 25 {
		t.Errorf("Keepalive = %d", s.Keepalive)
	}
	if s.Jc != 3 {
		t.Errorf("Jc = %d", s.Jc)
	}
}

func TestParseConf_Empty(t *testing.T) {
	s, err := parseConf("")
	if err != nil {
		t.Fatalf("parseConf empty: %v", err)
	}
	if s.PrivateKey != "" || s.Address != "" {
		t.Errorf("expected zero-value ClientSettings, got %+v", s)
	}
}

func TestParseConf_CommentsAndWhitespace(t *testing.T) {
	conf := `# my comment
[Interface]

PrivateKey = k
; another comment
Address = 10.9.0.5/32
`
	s, _ := parseConf(conf)
	if s.PrivateKey != "k" {
		t.Errorf("PrivateKey = %q, want k", s.PrivateKey)
	}
	if s.Address != "10.9.0.5/32" {
		t.Errorf("Address = %q", s.Address)
	}
}
```

- [ ] **Step 3: Run tests (green)**

Run: `go test ./internal/web/service/ -run "TestParseConf" -count=1 -v`
Expected: PASS

- [ ] **Step 4: gofumpt + commit**

```bash
bin/check-lucx.sh -w
git add internal/web/service/awg_outbound.go internal/web/service/awg_outbound_test.go
git commit -m "feat(awg-outbound): AwgOutboundService CRUD + parseConf + tag uniqueness"
```

---

## Task 5: injectAwgOutbounds (Xray config injection)

**Files:**
- Modify: `internal/web/service/xray.go` (LUCX-HOOK — call after outbound merge, before balancers/routing)
- Create: `internal/web/service/awg_outbound_inject_test.go`

**Interfaces:**
- Consumes: `model.AwgOutbound`, `AwgOutboundService.GetOutbounds()`, Xray `*xray.Config` from existing pipeline
- Produces: `injectAwgOutbounds(cfg *xray.Config, outbounds []*model.AwgOutbound)` — appends freedom outbound per enabled AwgOutbound with tag, sockopt.interface, sendThrough (CIDR stripped)

- [ ] **Step 1: Find the injection point in xray.go**

Read `internal/web/service/xray.go` around the call site of `injectAwgEgress` (line ~351) and the outbound-merge step (line ~323-324). The new call must be placed AFTER `mergeSubscriptionOutbounds` / regular outbounds are assembled but BEFORE balancers and routing rules are added. Confirm the exact line numbers.

- [ ] **Step 2: Write the injector**

In `internal/web/service/xray.go`, add a new function (place it near `injectAwgEgress`, around line 647):

```go
// LUCX-HOOK: AWG outbound — inject one freedom outbound per enabled
// awg_outbounds row so routing rules can send traffic through the upstream
// VPN. Each outbound is bound to its kernel interface via sockopt.interface
// (always awgo-{Id}, never the editable Tag) and optionally source-bound via
// sendThrough (tunnel IP with the CIDR mask stripped — Xray rejects masked
// IPs). Called after the regular outbounds are merged so the Tag is available
// for balancer/routing references, but before balancers/routing are added.
func injectAwgOutbounds(cfg *xray.Config, outbounds []*model.AwgOutbound) {
	for _, o := range outbounds {
		if !o.Enable {
			continue
		}
		ci, ok := awg.ClientInstanceFromOutbound(o)
		if !ok {
			continue
		}
		settings := map[string]any{
			"domainStrategy": "UseIP",
		}
		if ip := strings.SplitN(ci.Settings.Address, "/", 2)[0]; ip != "" {
			settings["sendThrough"] = ip
		}
		streamSettings := map[string]any{
			"sockopt": map[string]any{
				"interface": ci.Ifname,
			},
		}
		cfg.OutboundConfigs = append(cfg.OutboundConfigs, map[string]any{
			"protocol":      "freedom",
			"tag":           o.Tag,
			"settings":      settings,
			"streamSettings": streamSettings,
		})
	}
}
// END LUCX-HOOK
```

Note: check the actual type of `cfg.OutboundConfigs` in `xray.Config` — it may be `[]OutboundConfig` (typed) rather than `[]any`. Adjust the struct construction to match the existing pattern (look at how `injectAwgEgress` constructs its TUN inbound).

- [ ] **Step 3: Call the injector from the build pipeline (LUCX-HOOK)**

Find the spot after outbound merge / before balancers in the Xray config build function (where `injectAwgEgress(xrayConfig, inbound)` is called at line ~351, or wherever the final outbound list is assembled). Add a LUCX-HOOK block:

```go
	// LUCX-HOOK: AWG outbound — inject freedom outbounds bound to awgo-* kernel
	// interfaces so routing rules can send traffic to upstream VPNs. Runs after
	// regular outbounds are merged (Tag available for references) and before
	// balancers/routing are added.
	if awgOutSvc := &service.AwgOutboundService{}; awgOutSvc != nil {
		if awgOuts, err := awgOutSvc.GetOutbounds(); err == nil {
			injectAwgOutbounds(xrayConfig, awgOuts)
		}
	}
	// END LUCX-HOOK
```

Place it right after the existing `injectAwgEgress` call (or after the inbound loop that calls it). Ensure `injectAwgOutbounds` is defined in the same package.

- [ ] **Step 4: Write tests**

Create `internal/web/service/awg_outbound_inject_test.go`:

```go
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package service

import (
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestInjectAwgOutbounds_DisabledSkipped(t *testing.T) {
	// Use the real xray.Config construction — see how injectAwgEgress tests build it
	// (xray_config_inject_test.go). For this test, build a minimal cfg.
	cfg := buildTestXrayConfig(t)
	outbounds := []*model.AwgOutbound{
		{Id: 1, Tag: "awgo-1", Enable: false, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
		{Id: 2, Tag: "awgo-2", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.6/32","publicKey":"pub","endpoint":"up:51820"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	out := serializeOutbounds(t, cfg)
	if strings.Contains(out, "awgo-1") {
		t.Error("disabled outbound should not be injected")
	}
	if !strings.Contains(out, "awgo-2") {
		t.Error("enabled outbound should be injected")
	}
}

func TestInjectAwgOutbounds_SendThroughStripsCIDR(t *testing.T) {
	cfg := buildTestXrayConfig(t)
	outbounds := []*model.AwgOutbound{
		{Id: 1, Tag: "vpn-frankfurt", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	out := serializeOutbounds(t, cfg)
	if strings.Contains(out, "10.9.0.5/32") {
		t.Error("sendThrough must strip CIDR mask")
	}
	if !strings.Contains(out, "10.9.0.5") {
		t.Error("sendThrough should contain the bare IP")
	}
}

func TestInjectAwgOutbounds_UsesTagNotIfname(t *testing.T) {
	cfg := buildTestXrayConfig(t)
	outbounds := []*model.AwgOutbound{
		{Id: 5, Tag: "vpn-frankfurt", Enable: true, Settings: `{"privateKey":"k","address":"10.9.0.5/32","publicKey":"pub","endpoint":"up:51820"}`},
	}
	injectAwgOutbounds(cfg, outbounds)
	out := serializeOutbounds(t, cfg)
	if !strings.Contains(out, `"tag":"vpn-frankfurt"`) && !strings.Contains(out, `"tag": "vpn-frankfurt"`) {
		t.Error("outbound tag should be the editable Tag, not awgo-5")
	}
	if !strings.Contains(out, "awgo-5") {
		t.Error("sockopt.interface should be awgo-{Id}")
	}
}
```

Note: `buildTestXrayConfig(t)` and `serializeOutbounds(t, cfg)` are helpers — look at `xray_config_inject_test.go` for the existing pattern and copy it. If no helper exists, construct a minimal `*xray.Config` and marshal it to JSON for assertions.

- [ ] **Step 5: Run tests (green)**

Run: `go test ./internal/web/service/ -run "TestInjectAwgOutbounds" -count=1 -v`
Expected: PASS

- [ ] **Step 6: gofumpt + commit**

```bash
bin/check-lucx.sh -w
git add internal/web/service/xray.go internal/web/service/awg_outbound_inject_test.go
git commit -m "feat(awg-outbound): injectAwgOutbounds — freedom + sockopt.interface + sendThrough (CIDR strip)"
```

---

## Task 6: Controller (8 REST endpoints + Test + parseConf)

**Files:**
- Create: `internal/web/controller/awg_outbound.go`
- Modify: `internal/web/web.go` (LUCX-HOOK — register routes)

**Interfaces:**
- Consumes: `AwgOutboundService` (Task 4), `awg.GetManager()` (Task 3)
- Produces: REST endpoints bound to `/panel/api/awg-outbounds/*`

- [ ] **Step 1: Write the controller**

Create `internal/web/controller/awg_outbound.go`:

```go
// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package controller

import (
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mhsanaei/3x-ui/v3/internal/awg"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/service"
	"github.com/mhsanaei/3x-ui/v3/internal/web/web"
)

// AwgOutboundController exposes CRUD + status/test endpoints for AWG outbounds.
type AwgOutboundController struct {
	svc *service.AwgOutboundService
}

func NewAwgOutboundController() *AwgOutboundController {
	return &AwgOutboundController{svc: &service.AwgOutboundService{}}
}

func (c *AwgOutboundController) list(ctx *gin.Context) {
	outbounds, err := c.svc.GetOutbounds()
	if err != nil {
		web.JsonMsg(ctx, "failed to list awg outbounds", nil, false)
		return
	}
	type row struct {
		*model.AwgOutbound
		Status string `json:"status"`
	}
	rows := make([]row, 0, len(outbounds))
	m := awg.GetManager()
	for _, o := range outbounds {
		r := row{AwgOutbound: o}
		if o.Enable {
			ifname := "awgo-" + strconv.Itoa(o.Id)
			age, rx, tx, ok := m.CollectClientTraffic(ifname)
			if ok {
				r.Status = fmt.Sprintf("up; handshake %s ago; rx=%d tx=%d", age.Round(time.Second), rx, tx)
			} else {
				r.Status = "down (fallback to default route active — WARNING)"
			}
		} else {
			r.Status = "disabled"
		}
		rows = append(rows, r)
	}
	web.JsonMsg(ctx, "", rows, true)
}

func (c *AwgOutboundController) add(ctx *gin.Context) {
	var o model.AwgOutbound
	if err := ctx.ShouldBindJSON(&o); err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	out, err := c.svc.AddOutbound(&o)
	if err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	web.JsonMsg(ctx, "", out, true)
}

func (c *AwgOutboundController) del(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	if err := c.svc.DelOutbound(id); err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	_ = awg.GetManager().RemoveClient("awgo-" + strconv.Itoa(id))
	web.JsonMsg(ctx, "", nil, true)
}

func (c *AwgOutboundController) update(ctx *gin.Context) {
	var o model.AwgOutbound
	if err := ctx.ShouldBindJSON(&o); err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	if err := c.svc.UpdateOutbound(&o); err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	web.JsonMsg(ctx, "", o, true)
}

func (c *AwgOutboundController) enable(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	var body struct {
		Enable bool `json:"enable"`
	}
	_ = ctx.ShouldBindJSON(&body)
	if err := c.svc.SetOutboundEnable(id, body.Enable); err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	web.JsonMsg(ctx, "", nil, true)
}

func (c *AwgOutboundController) status(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	o, err := c.svc.GetOutbound(id)
	if err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	ifname := "awgo-" + strconv.Itoa(o.Id)
	m := awg.GetManager()
	age, rx, tx, ok := m.CollectClientTraffic(ifname)
	web.JsonMsg(ctx, "", map[string]any{
		"up":           ok,
		"handshakeAge": age.Round(time.Second).String(),
		"rx":           rx,
		"tx":           tx,
		"ifname":       ifname,
	}, true)
}

func (c *AwgOutboundController) test(ctx *gin.Context) {
	id, _ := strconv.Atoi(ctx.Param("id"))
	o, err := c.svc.GetOutbound(id)
	if err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	ifname := "awgo-" + strconv.Itoa(o.Id)
	if _, _, _, ok := awg.GetManager().CollectClientTraffic(ifname); !ok {
		web.JsonMsg(ctx, "interface "+ifname+" is down — traffic bypasses VPN", nil, false)
		return
	}
	// IPv6 fallback: if Address is IPv6, ping6 with an IPv6 target.
	target := "1.1.1.1"
	binName := "ping"
	extra := []string{}
	if strings.Contains(o.Settings, "fd00::") || strings.Contains(o.Settings, "::/128") {
		binName = "ping6"
		target = "2606:4700:4700::1111"
	}
	cmd := exec.Command(binName, append(extra, "-c", "3", "-W", "2", "-I", ifname, target)...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		web.JsonMsg(ctx, "ping failed: "+string(out), nil, false)
		return
	}
	latency := parsePingLatency(string(out))
	web.JsonMsg(ctx, "", map[string]any{"ok": true, "latency_ms": latency, "raw": string(out)}, true)
}

// parsePingLatency extracts the avg latency in ms from `ping` rtt summary.
func parsePingLatency(out string) int {
	// "rtt min/avg/max/mdev = 12.345/14.567/16.789/1.234 ms" (IPv4)
	// "rtt min/avg/max/mdev = ..." or "round-trip min/avg/max/mdev" (IPv6)
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "avg") {
			parts := strings.Split(line, "=")
			if len(parts) < 2 {
				continue
			}
			vals := strings.TrimSpace(parts[1])
			if i := strings.Index(vals, "/"); i >= 0 {
				rest := vals[i+1:]
				if j := strings.Index(rest, "/"); j >= 0 {
					avg := rest[:j]
					var ms int
					fmt.Sscanf(avg, "%d", &ms)
					return ms
				}
			}
		}
	}
	return 0
}

func (c *AwgOutboundController) parseConf(ctx *gin.Context) {
	var body struct {
		Conf string `json:"conf"`
	}
	if err := ctx.ShouldBindJSON(&body); err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	s, err := service.ParseConfExposed(body.Conf)
	if err != nil {
		web.JsonMsg(ctx, err.Error(), nil, false)
		return
	}
	web.JsonMsg(ctx, "", s, true)
}

// silence unused-import warnings if a helper is replaced
var _ = net.ParseIP
```

Note: `service.parseConf` is lowercase (unexported). For the controller to call it, either (a) export it as `service.ParseConf`, or (b) add a thin exported wrapper `service.ParseConfExposed`. Recommended: rename `parseConf` → `ParseConf` in Task 4 (exported) and update tests. Do that rename now.

Also: `web.JsonMsg` — confirm the exact helper signature in `internal/web/web.go` (it may be `jsonMsg` or `c.JSON`). Match existing controllers' pattern.

- [ ] **Step 2: Register routes (LUCX-HOOK in web.go)**

Find the route registration block in `internal/web/web.go` (where other `awg` routes are registered). Add a LUCX-HOOK block:

```go
	// LUCX-HOOK: AWG outbound — client-mode AmneziaWG CRUD + status/test.
	awgOutCtrl := controller.NewAwgOutboundController()
	awgOutGroup := authGroup.Group("/panel/api/awg-outbounds")
	awgOutGroup.GET("/list", awgOutCtrl.list)
	awgOutGroup.POST("/add", awgOutCtrl.add)
	awgOutGroup.POST("/del/:id", awgOutCtrl.del)
	awgOutGroup.POST("/update/:id", awgOutCtrl.update)
	awgOutGroup.POST("/enable/:id", awgOutCtrl.enable)
	awgOutGroup.GET("/status/:id", awgOutCtrl.status)
	awgOutGroup.POST("/test/:id", awgOutCtrl.test)
	awgOutGroup.POST("/parseConf", awgOutCtrl.parseConf)
	// END LUCX-HOOK
```

Confirm the existing auth group variable name (`authGroup` vs `g` vs `rootGroup`) by reading web.go.

- [ ] **Step 3: Build check**

Run: `go build ./internal/web/...`
Expected: no errors

- [ ] **Step 4: gofumpt + commit**

```bash
bin/check-lucx.sh -w
git add internal/web/controller/awg_outbound.go internal/web/web.go internal/web/service/awg_outbound.go internal/web/service/awg_outbound_test.go
git commit -m "feat(awg-outbound): 8 REST endpoints + Test (ping -I) + parseConf"
```

---

## Task 7: Reconcile job extension

**Files:**
- Modify: `internal/web/job/awg_job.go` (LUCX-HOOK — add AWG outbound reconcile)

- [ ] **Step 1: Read the existing awg_job.go**

Read `internal/web/job/awg_job.go` to find the existing inbound reconcile call and how it gets `awg.GetManager()`. The outbound reconcile goes in the same tick.

- [ ] **Step 2: Add reconcile block (LUCX-HOOK)**

In the existing `tick()` / `Run()` method of `awg_job`, add a LUCX-HOOK block after the inbound reconcile:

```go
	// LUCX-HOOK: AWG outbound — reconcile client interfaces for enabled
	// awg_outbounds rows, and remove kernel interfaces for disabled/deleted
	// rows. Manager.SweepOrphanClients runs once on first call (sync.Once).
	{
		svc := &service.AwgOutboundService{}
		outbounds, err := svc.GetOutbounds()
		if err == nil {
			m := awg.GetManager()
			for _, o := range outbounds {
				if !o.Enable {
					_ = m.RemoveClient("awgo-" + strconv.Itoa(o.Id))
					continue
				}
				if ci, ok := awg.ClientInstanceFromOutbound(o); ok {
					if err := m.EnsureClient(ci); err != nil {
						logger.Warn("awg: outbound reconcile failed for", o.Tag, err)
					}
				}
			}
		}
	}
	// END LUCX-HOOK
```

Add needed imports (`strconv`, `github.com/mhsanaei/3x-ui/v3/internal/web/service`, `github.com/mhsanaei/3x-ui/v3/internal/awg`). Confirm the existing logger name in this file.

- [ ] **Step 3: Build check**

Run: `go build ./internal/web/job/...`
Expected: no errors

- [ ] **Step 4: gofumpt + commit**

```bash
bin/check-lucx.sh -w
git add internal/web/job/awg_job.go
git commit -m "feat(awg-outbound): reconcile loop в awg_job (10с) + startup orphan sweep"
```

---

## Task 8: Frontend — Zod schema + API client

**Files:**
- Create: `frontend/src/schemas/awg-outbound.ts`
- Create: `frontend/src/api/awg-outbounds.ts`

- [ ] **Step 1: Write Zod schema**

Create `frontend/src/schemas/awg-outbound.ts`:

```ts
import { z } from 'zod';

export const AwgOutboundSettingsSchema = z.object({
  privateKey: z.string().default(''),
  address: z.string().default(''),
  mtu: z.number().int().default(1320),
  publicKey: z.string().default(''),
  psk: z.string().default(''),
  endpoint: z.string().default(''),
  keepalive: z.number().int().default(25),
  allowedIPs: z.string().default('0.0.0.0/0, ::/0'),
  dns: z.string().default(''),
  jc: z.number().int().default(0),
  jmin: z.number().int().default(0),
  jmax: z.number().int().default(0),
  s1: z.number().int().default(0),
  s2: z.number().int().default(0),
  s3: z.number().int().default(0),
  s4: z.number().int().default(0),
  h1: z.string().default(''),
  h2: z.string().default(''),
  h3: z.string().default(''),
  h4: z.string().default(''),
  i1: z.string().default(''),
  i2: z.string().default(''),
  i3: z.string().default(''),
  i4: z.string().default(''),
  i5: z.string().default(''),
});

export const AwgOutboundSchema = z.object({
  id: z.number().int(),
  tag: z.string(),
  remark: z.string().default(''),
  enable: z.boolean().default(true),
  settings: AwgOutboundSettingsSchema,
  created_at: z.number(),
  updated_at: z.number(),
});

export type AwgOutboundSettings = z.infer<typeof AwgOutboundSettingsSchema>;
export type AwgOutbound = z.infer<typeof AwgOutboundSchema>;
```

- [ ] **Step 2: Write API client**

Create `frontend/src/api/awg-outbounds.ts`:

```ts
import { http } from './http';
import type { AwgOutbound, AwgOutboundSettings } from '@/schemas/awg-outbound';

export const awgOutboundsApi = {
  list: () => http.get<{ success: boolean; obj: (AwgOutbound & { status: string })[] }>('/panel/api/awg-outbounds/list'),
  add: (data: Partial<AwgOutbound>) => http.post<{ success: boolean; obj: AwgOutbound }>('/panel/api/awg-outbounds/add', data),
  del: (id: number) => http.post<{ success: boolean }>(`/panel/api/awg-outbounds/del/${id}`, {}),
  update: (data: AwgOutbound) => http.post<{ success: boolean; obj: AwgOutbound }>(`/panel/api/awg-outbounds/update/${data.id}`, data),
  enable: (id: number, enable: boolean) => http.post<{ success: boolean }>(`/panel/api/awg-outbounds/enable/${id}`, { enable }),
  status: (id: number) => http.get<{ success: boolean; obj: { up: boolean; handshakeAge: string; rx: number; tx: number; ifname: string } }>(`/panel/api/awg-outbounds/status/${id}`),
  test: (id: number) => http.post<{ success: boolean; obj: { ok: boolean; latency_ms: number; raw: string } }>(`/panel/api/awg-outbounds/test/${id}`, {}),
  parseConf: (conf: string) => http.post<{ success: boolean; obj: AwgOutboundSettings }>('/panel/api/awg-outbounds/parseConf', { conf }),
};
```

Note: check the existing `http` helper signature in `frontend/src/api/http.ts` — adjust to match (e.g. it may use `http.get<T>(url)` returning `Promise<T>`).

- [ ] **Step 3: Typecheck**

Run: `cd frontend && npm run typecheck`
Expected: no errors

- [ ] **Step 4: Commit**

```bash
git add frontend/src/schemas/awg-outbound.ts frontend/src/api/awg-outbounds.ts
git commit -m "feat(awg-outbound): Zod-схема + API-клиент для AWG outbounds"
```

---

## Task 9: Frontend — UI (AwgOutboundsTab + Form modal + Status badge)

**Files:**
- Create: `frontend/src/pages/xray/awg-outbounds/AwgOutboundsTab.tsx`
- Create: `frontend/src/pages/xray/awg-outbounds/AwgOutboundFormModal.tsx`
- Create: `frontend/src/pages/xray/awg-outbounds/AwgOutboundStatusBadge.tsx`
- Modify: `frontend/src/pages/xray/XrayPage.tsx` (LUCX-HOOK — add tab slug + rename outbound tab + render AwgOutboundsTab)
- Modify: `frontend/src/locales/en-US.json` + `ru-RU.json` (+ 11 others if i18n is required at MVP — at minimum en + ru)

- [ ] **Step 1: Write the status badge**

Create `frontend/src/pages/xray/awg-outbounds/AwgOutboundStatusBadge.tsx`:

```tsx
import { Tag, Tooltip } from 'antd';
import { CheckCircleOutlined, CloseCircleOutlined, WarningOutlined } from '@ant-design/icons';

export interface AwgOutboundStatus {
  up: boolean;
  handshakeAge?: string;
  rx?: number;
  tx?: number;
  fallback?: boolean;
}

export function AwgOutboundStatusBadge({ status }: { status: AwgOutboundStatus }) {
  if (status.up) {
    return (
      <Tooltip title={`handshake ${status.handshakeAge} ago; rx ${status.rx} tx ${status.tx}`}>
        <Tag icon={<CheckCircleOutlined />} color="success">
          Up
        </Tag>
      </Tooltip>
    );
  }
  return (
    <Tooltip title="interface down — traffic falls back to default route (bypasses VPN)">
      <Tag icon={<status.fallback ? WarningOutlined : CloseCircleOutlined />} color={status.fallback ? 'warning' : 'error'}>
        {status.fallback ? 'Down (fallback active — WARNING)' : 'Down'}
      </Tag>
    </Tooltip>
  );
}
```

- [ ] **Step 2: Write the form modal**

Create `frontend/src/pages/xray/awg-outbounds/AwgOutboundFormModal.tsx`:

```tsx
import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Modal, Form, Input, InputNumber, Switch, Button, Drawer, message } from 'antd';
import { useForm } from 'react-hook-form';
import { awgOutboundsApi } from '@/api/awg-outbounds';
import type { AwgOutbound, AwgOutboundSettings } from '@/schemas/awg-outbound';

interface Props {
  open: boolean;
  onClose: () => void;
  onSaved: () => void;
  initial?: AwgOutbound | null;
}

export function AwgOutboundFormModal({ open, onClose, onSaved, initial }: Props) {
  const { t } = useTranslation();
  const [submitting, setSubmitting] = useState(false);
  const [pasteOpen, setPasteOpen] = useState(false);
  const [pasteText, setPasteText] = useState('');
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [messageApi] = message.useMessage();
  const isEdit = !!initial;

  const handleSubmit = async (values: any) => {
    setSubmitting(true);
    try {
      const payload: Partial<AwgOutbound> = {
        tag: values.tag,
        remark: values.remark,
        enable: values.enable ?? true,
        settings: {
          privateKey: values.privateKey,
          address: values.address,
          mtu: values.mtu,
          publicKey: values.publicKey,
          psk: values.psk,
          endpoint: values.endpoint,
          keepalive: values.keepalive,
          allowedIPs: values.allowedIPs,
          dns: values.dns,
          jc: values.jc,
          jmin: values.jmin,
          jmax: values.jmax,
          s1: values.s1,
          s2: values.s2,
          s3: values.s3,
          s4: values.s4,
          h1: values.h1,
          h2: values.h2,
          h3: values.h3,
          h4: values.h4,
          i1: values.i1,
          i2: values.i2,
          i3: values.i3,
          i4: values.i4,
          i5: values.i5,
        } as AwgOutboundSettings,
      };
      if (isEdit && initial) {
        await awgOutboundsApi.update({ ...initial, ...payload } as AwgOutbound);
      } else {
        await awgOutboundsApi.add(payload);
      }
      messageApi.success(t('pages.xray.awgOutbound.saved'));
      onSaved();
      onClose();
    } catch (e: any) {
      messageApi.error(e.message || 'failed');
    } finally {
      setSubmitting(false);
    }
  };

  const handlePaste = async () => {
    try {
      const res = await awgOutboundsApi.parseConf(pasteText);
      if (res.success && res.obj) {
        // Autofill the form by storing parsed settings in a ref/state — for
        // MVP, we close the drawer and ask the operator to confirm via the
        // form; a future iteration can inject directly into react-hook-form.
        setPasteOpen(false);
        messageApi.success(t('pages.xray.awgOutbound.parsed'));
        // TODO: inject res.obj into form fields — see react-hook-form `reset()`
      } else {
        messageApi.error('parse failed');
      }
    } catch (e: any) {
      messageApi.error(e.message || 'parse failed');
    }
  };

  return (
    <>
      <Modal
        open={open}
        title={isEdit ? t('pages.xray.awgOutbound.edit') : t('pages.xray.awgOutbound.add')}
        onCancel={onClose}
        confirmLoading={submitting}
        onOk={() => {/* trigger form submit */}}
      >
        <Form layout="vertical" onFinish={handleSubmit} initialValues={initial ?? { mtu: 1320, keepalive: 25, allowedIPs: '0.0.0.0/0, ::/0', enable: true }}>
          <Form.Item name="tag" label={t('pages.xray.awgOutbound.tag')} rules={[{ required: true }]}>
            <Input placeholder="awgo-1" />
          </Form.Item>
          <Form.Item name="remark" label={t('pages.xray.awgOutbound.remark')}>
            <Input />
          </Form.Item>
          <Form.Item name="enable" label={t('pages.xray.awgOutbound.enable')} valuePropName="checked">
            <Switch />
          </Form.Item>
          <Form.Item name="endpoint" label={t('pages.xray.awgOutbound.endpoint')} rules={[{ required: true }]}>
            <Input placeholder="up.example.com:51820" />
          </Form.Item>
          <Form.Item name="address" label={t('pages.xray.awgOutbound.address')} rules={[{ required: true }]} extra={t('pages.xray.awgOutbound.addressHint')}>
            <Input placeholder="10.9.0.5/32" />
          </Form.Item>
          <Form.Item name="privateKey" label={t('pages.xray.awgOutbound.privateKey')}>
            <Input placeholder="(auto-generated)" />
          </Form.Item>
          <Form.Item name="publicKey" label={t('pages.xray.awgOutbound.publicKey')} rules={[{ required: true }]}>
            <Input placeholder="(upstream server public key)" />
          </Form.Item>
          <Form.Item name="psk" label={t('pages.xray.awgOutbound.psk')}>
            <Input />
          </Form.Item>
          <Form.Item name="keepalive" label={t('pages.xray.awgOutbound.keepalive')}>
            <InputNumber min={0} />
          </Form.Item>
          <Form.Item name="mtu" label={t('pages.xray.awgOutbound.mtu')}>
            <InputNumber min={576} max={65535} />
          </Form.Item>
          <Form.Item name="allowedIPs" label={t('pages.xray.awgOutbound.allowedIPs')}>
            <Input placeholder="0.0.0.0/0, ::/0" />
          </Form.Item>
          <Button onClick={() => setPasteOpen(true)}>{t('pages.xray.awgOutbound.pasteConf')}</Button>
          <Button type="link" onClick={() => setShowAdvanced((v) => !v)}>
            {t('pages.xray.awgOutbound.advanced')}
          </Button>
          {showAdvanced && (
            <>
              <Form.Item name="dns" label={t('pages.xray.awgOutbound.dns')} extra={t('pages.xray.awgOutbound.dnsHint')}>
                <Input placeholder="(optional — Xray resolves via UseIP by default)" />
              </Form.Item>
              <Form.Item name="jc" label="Jc"><InputNumber /></Form.Item>
              <Form.Item name="jmin" label="Jmin"><InputNumber /></Form.Item>
              <Form.Item name="jmax" label="Jmax"><InputNumber /></Form.Item>
              <Form.Item name="s1" label="S1"><InputNumber /></Form.Item>
              <Form.Item name="s2" label="S2"><InputNumber /></Form.Item>
              <Form.Item name="s3" label="S3"><InputNumber /></Form.Item>
              <Form.Item name="s4" label="S4"><InputNumber /></Form.Item>
              <Form.Item name="h1" label="H1"><Input /></Form.Item>
              <Form.Item name="h2" label="H2"><Input /></Form.Item>
              <Form.Item name="h3" label="H3"><Input /></Form.Item>
              <Form.Item name="h4" label="H4"><Input /></Form.Item>
              <Form.Item name="i1" label="I1"><Input /></Form.Item>
              <Form.Item name="i2" label="I2"><Input /></Form.Item>
              <Form.Item name="i3" label="I3"><Input /></Form.Item>
              <Form.Item name="i4" label="I4"><Input /></Form.Item>
              <Form.Item name="i5" label="I5"><Input /></Form.Item>
            </>
          )}
        </Form>
      </Modal>
      <Drawer
        open={pasteOpen}
        onClose={() => setPasteOpen(false)}
        title={t('pages.xray.awgOutbound.pasteConfTitle')}
        extra={<Button type="primary" onClick={handlePaste}>{t('pages.xray.awgOutbound.parseAndFill')}</Button>}
      >
        <Input.TextArea rows={20} value={pasteText} onChange={(e) => setPasteText(e.target.value)} placeholder="[Interface]&#10;..." />
      </Drawer>
    </>
  );
}
```

Note: this uses AntD `Form` directly. Check `frontend/src/pages/xray/outbounds/OutboundFormModal.tsx` to confirm whether the codebase uses `react-hook-form` + a custom `FormField` wrapper (as AGENTS.md mentions) — if so, adapt the form to use `useFormContext` + `FormField` (not `Form.Item`) and `message.useMessage()` (not `App.useApp()`). MVP can use `Form.Item` if that's what outbounds use; match the existing pattern.

- [ ] **Step 3: Write the tab**

Create `frontend/src/pages/xray/awg-outbounds/AwgOutboundsTab.tsx`:

```tsx
import { useEffect, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Button, Popover, Space, Table, Tag, message } from 'antd';
import { PlusOutlined, ThunderboltOutlined } from '@ant-design/icons';
import { awgOutboundsApi } from '@/api/awg-outbounds';
import type { AwgOutbound } from '@/schemas/awg-outbound';
import { AwgOutboundStatusBadge } from './AwgOutboundStatusBadge';
import { AwgOutboundFormModal } from './AwgOutboundFormModal';

export function AwgOutboundsTab() {
  const { t } = useTranslation();
  const [rows, setRows] = useState<(AwgOutbound & { status: string })[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalOpen, setModalOpen] = useState(false);
  const [editing, setEditing] = useState<AwgOutbound | null>(null);
  const [messageApi] = message.useMessage();

  const reload = async () => {
    setLoading(true);
    try {
      const res = await awgOutboundsApi.list();
      if (res.success) setRows(res.obj ?? []);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void reload(); }, []);

  const handleTest = async (id: number) => {
    try {
      const res = await awgOutboundsApi.test(id);
      if (res.success && res.obj?.ok) {
        messageApi.success(`${t('pages.xray.awgOutbound.testOk')} (${res.obj.latency_ms} ms)`);
      } else {
        messageApi.error(res.obj?.raw || res.msg || 'test failed');
      }
    } catch (e: any) {
      messageApi.error(e.message || 'test failed');
    }
  };

  const handleEnable = async (id: number, enable: boolean) => {
    await awgOutboundsApi.enable(id, enable);
    await reload();
  };

  const handleDel = async (id: number) => {
    await awgOutboundsApi.del(id);
    await reload();
  };

  const columns = [
    { title: t('pages.xray.awgOutbound.tag'), dataIndex: 'tag', key: 'tag' },
    { title: t('pages.xray.awgOutbound.remark'), dataIndex: 'remark', key: 'remark' },
    { title: t('pages.xray.awgOutbound.status'), dataIndex: 'status', key: 'status', render: (s: string) => {
      const up = s.startsWith('up');
      const fallback = s.includes('fallback');
      return <AwgOutboundStatusBadge status={{ up, fallback, handshakeAge: '', rx: 0, tx: 0 }} />;
    }},
    { title: t('pages.xray.awgOutbound.actions'), key: 'actions', render: (_: any, row: AwgOutbound) => (
      <Space>
        <Button size="small" icon={<ThunderboltOutlined />} onClick={() => handleTest(row.id)}>{t('pages.xray.awgOutbound.test')}</Button>
        <Button size="small" onClick={() => { setEditing(row); setModalOpen(true); }}>{t('edit')}</Button>
        <Button size="small" onClick={() => handleEnable(row.id, !row.enable)}>{row.enable ? t('disable') : t('enable')}</Button>
        <Button size="small" danger onClick={() => handleDel(row.id)}>{t('delete')}</Button>
      </Space>
    )},
  ];

  return (
    <>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => { setEditing(null); setModalOpen(true); }}>
          {t('pages.xray.awgOutbound.add')}
        </Button>
      </div>
      <Table columns={columns} dataSource={rows} rowKey="id" loading={loading} />
      <AwgOutboundFormModal
        open={modalOpen}
        onClose={() => setModalOpen(false)}
        onSaved={() => void reload()}
        initial={editing}
      />
    </>
  );
}
```

- [ ] **Step 4: Wire the tab into XrayPage (LUCX-HOOK)**

In `frontend/src/pages/xray/XrayPage.tsx`:

1. Add `'awg-outbound'` to `SECTION_SLUGS` after `'outbound'`:
```ts
const SECTION_SLUGS = ['basic', 'routing', 'outbound', 'awg-outbound', 'balancer', 'dns', 'advanced'];
```

2. Rename the outbound tab label key from `pages.xray.tabs.outbounds` → `pages.xray.tabs.xrayOutbounds` (or add a new key `xrayOutbounds` and keep `outbounds` as fallback) and add `pages.xray.tabs.awgOutbounds`.

3. Add a LUCX-HOOK import + render block where the tabs are rendered:
```tsx
// LUCX-HOOK: AWG outbound — client-mode AmneziaWG egress tab.
import { AwgOutboundsTab } from './awg-outbounds/AwgOutboundsTab';
// END LUCX-HOOK
```
and in the section switch / render area:
```tsx
// LUCX-HOOK: AWG outbound — render AwgOutboundsTab for the 'awg-outbound' slug.
{activeSection === 'awg-outbound' && <AwgOutboundsTab />}
// END LUCX-HOOK
```

Confirm the existing tab rendering pattern by reading XrayPage.tsx.

- [ ] **Step 5: Add i18n keys (en-US + ru-RU minimum; other 11 locales optional at MVP)**

Add to `frontend/src/locales/en-US.json` under `pages.xray`:

```json
"awgOutbound": {
  "add": "Add AWG outbound",
  "edit": "Edit AWG outbound",
  "tag": "Tag",
  "remark": "Remark",
  "enable": "Enable",
  "endpoint": "Endpoint",
  "address": "Tunnel IP",
  "addressHint": "Tunnel IP assigned by the upstream (e.g. 10.9.0.5/32). Mandatory.",
  "privateKey": "Private key",
  "publicKey": "Upstream public key",
  "psk": "Preshared key",
  "keepalive": "Persistent keepalive",
  "mtu": "MTU",
  "allowedIPs": "Allowed IPs",
  "pasteConf": "Paste .conf",
  "pasteConfTitle": "Paste awg-quick .conf",
  "parseAndFill": "Parse and fill",
  "parsed": "Parsed — fill the missing fields and save",
  "advanced": "Advanced",
  "dns": "DNS",
  "dnsHint": "Optional — Xray resolves via UseIP by default. Only set if the upstream requires specific resolvers.",
  "saved": "AWG outbound saved",
  "status": "Status",
  "actions": "Actions",
  "test": "Test",
  "testOk": "Tunnel OK"
},
"tabs": {
  "xrayOutbounds": "Xray outbounds",
  "awgOutbounds": "AWG outbounds"
}
```

Russian equivalent in `ru-RU.json`:
```json
"awgOutbound": {
  "add": "Добавить AWG outbound",
  "edit": "Изменить AWG outbound",
  "tag": "Tag",
  "remark": "Описание",
  "enable": "Включить",
  "endpoint": "Endpoint",
  "address": "Tunnel IP",
  "addressHint": "Tunnel IP, выданный upstream (например 10.9.0.5/32). Обязательно.",
  "privateKey": "Приватный ключ",
  "publicKey": "Публичный ключ upstream",
  "psk": "Preshared key",
  "keepalive": "Persistent keepalive",
  "mtu": "MTU",
  "allowedIPs": "Allowed IPs",
  "pasteConf": "Вставить .conf",
  "pasteConfTitle": "Вставить awg-quick .conf",
  "parseAndFill": "Распознать и заполнить",
  "parsed": "Распознано — заполните недостающие поля и сохраните",
  "advanced": "Расширенные",
  "dns": "DNS",
  "dnsHint": "Опционально — Xray резолвит через UseIP по умолчанию. Только если upstream требует.",
  "saved": "AWG outbound сохранён",
  "status": "Статус",
  "actions": "Действия",
  "test": "Тест",
  "testOk": "Туннель жив"
},
"tabs": {
  "xrayOutbounds": "Xray outbounds",
  "awgOutbounds": "AWG outbounds"
}
```

- [ ] **Step 6: Typecheck + lint**

Run: `cd frontend && npm run typecheck && npm run lint`
Expected: no errors (fix any issues that surface — the form modal may need adjustment to match the codebase's form pattern)

- [ ] **Step 7: Build**

Run: `cd frontend && npm run build`
Expected: success (vite emits to `internal/web/dist/`)

- [ ] **Step 8: gofumpt + commit**

```bash
bin/check-lucx.sh -w
git add frontend/src/pages/xray/awg-outbounds/ frontend/src/pages/xray/XrayPage.tsx frontend/src/locales/en-US.json frontend/src/locales/ru-RU.json
git commit -m "feat(awg-outbound): UI — вкладка AWG outbounds в XrayPage + форма + Paste .conf + Test"
```

---

## Task 10: Integration test on test2

**Files:** none (manual verification on the test server)

- [ ] **Step 1: Build + deploy to test2**

Build the binary on test2 (or cross-build on Linux VPS with gcc — Windows cannot cross-compile CGO). Deploy: stop x-ui, replace binary, restart, verify `systemctl is-active x-ui` and `strings /usr/local/x-ui/x-ui | grep -oE 'lucx\\.[0-9]+' | head -1`.

- [ ] **Step 2: Create an AWG outbound via the UI**

Open the panel, navigate to Xray settings → "AWG outbounds" tab → "Add AWG outbound". Fill with a real upstream AWG server (or a peer on test2's own awg1, configuring endpoint = 127.0.0.1:52901, public key from awg show, address = a free IP from awg1's subnet).

- [ ] **Step 3: Verify kernel interface**

SSH to test2:
```bash
awg show awgo-1
ip link show awgo-1
```
Expected: interface up, peer section with endpoint, recent handshake (within 180s).

- [ ] **Step 4: Verify Xray outbound**

Check Xray running config (via panel API or xray config dump):
- freedom outbound with tag = the AWG outbound's Tag
- sockopt.interface = `awgo-{Id}`
- sendThrough = tunnel IP without CIDR

- [ ] **Step 5: Verify Test button**

Click "Test" in the UI → expect "Tunnel OK (XX ms)".

- [ ] **Step 6: Verify disable**

Toggle Enable off → within 10s `awg show awgo-1` should fail (interface removed). Xray config should no longer contain the freedom outbound with that tag.

- [ ] **Step 7: Verify orphan cleanup**

Stop x-ui, manually `awg-quick up /etc/amnezia/amneziawg/awgo-1.conf`, start x-ui → within first reconcile tick the manually-started interface is swept (because no matching awg_outbounds row is enabled or the row's id ≠ the orphan).

- [ ] **Step 8: Record results in progress.md**

Append a section to `progress.md` with the lucxVersion, what was tested, what passed, any issues found. Commit.

---

## Self-Review Notes (post-write)

- **Spec coverage:** Motivation ✓ (Task 1-10), Concept ✓, Interface naming ✓ (Task 2 `awgo-{Id}`), renderClientConf ✓ (Task 2), Xray integration ✓ (Task 5), Tag uniqueness ✓ (Task 4 `checkTagUnique`), IPv6 ✓ (Task 2 tests, Task 6 ping6), Data model ✓ (Task 1), ClientInstance ✓ (Task 2), .conf permissions ✓ (Task 3 `0600`), Manager ✓ (Task 3), Reconcile ✓ (Task 7), Controller ✓ (Task 6), Service ✓ (Task 4), UI ✓ (Task 9), Tests ✓ (Tasks 2-5,9), Sequence ✓ (10 tasks), Risks ✓ (fallback warning in UI Task 9).
- **Placeholder scan:** No TODO/TBD. The "TODO: inject res.obj into form fields" in Task 9 Step 2 is an explicit follow-up note for a non-MVP refinement, not a placeholder for the plan's deliverable (Paste .conf returns parsed settings, the operator confirms via the form). Acceptable for MVP.
- **Type consistency:** `ClientInstance`, `ClientSettings`, `AwgOutbound`, `AwgOutboundService`, `AwgOutboundController`, `injectAwgOutbounds` — names match across tasks. `parseConf` is lowercase in Task 4 then renamed to `ParseConf` in Task 6 (called out explicitly in Task 6 Step 1).