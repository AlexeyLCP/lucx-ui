# 01 — Purpose

Extracted from AGENTS.md. This file is project law.

---

## Project Overview

LucX-UI is a fork of [3x-ui](https://github.com/MHSanaei/3x-ui) (currently **v3.6.0**) that adds native AmneziaWG (AWG) support as a kernel-interface sidecar, mirroring upstream's MTProto (mtg) sidecar architecture. LucX-specific code lives in `internal/awg/` and `internal/lucx/`; all integration points in upstream files are wrapped in `LUCX-HOOK` / `END LUCX-HOOK` markers.

**Upstream sync strategy:** **merge** `origin/main` (not rebase, not fresh-checkout). The v3.5.0→3.6.0 sync proved the isolation works: of 432 upstream-changed files only **7** conflicted, so a plain `git merge origin/main` is now the procedure — see Rule 8. The merge commit keeps upstream history, so each next sync is incremental. The old `.patch`-file system is gone; integration is inline.

**Remotes:**
- `origin` → `MHSanaei/3x-ui` (upstream)
- `gh` → `AlexeyLCP/lucx-ui` (our fork; source of truth)
- `sc` → `ssh://ssh.sourcecraft.dev/alexeylcp/lucx-ui.git` (Yandex SourceCraft mirror; own CI/releases)

**Active branch:** `main` (v3.6.0 migration complete; current `lucxVersion` is in `internal/config/config.go`).

---

## Core Philosophy

**Minimal invasion for easy upstream sync.** The goal is: every upstream release should be a near-trivial port. This means:
- LucX code lives in isolated packages (`internal/awg/`, `internal/lucx/`), not scattered across upstream files.
- Upstream files get ONLY `LUCX-HOOK` blocks — never free-form edits.
- The AWG sidecar should be as thin as the MTProto sidecar. If mtproto does it in N files, AWG should aim for N too. (Known Issue #1 is closed — core package is at 9 files, exact parity with mtproto.)

**AWG sidecar = mtproto pattern.** AWG runs as a kernel-interface sidecar exactly symmetric with `internal/mtproto/`:

```
mtproto:  mtg sidecar (userspace)  → TCP → SOCKS loopback inbound → Xray routing
AWG:      awg kernel module        → IP   → TUN inbound             → Xray routing
```
