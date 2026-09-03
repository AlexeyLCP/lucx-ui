# 06 — Workflow

Extracted from AGENTS.md. This file is project law.

---

## Workflow: How an Agent Executes a Task

```
1. READ    → Read .agents/00-index.md, then 01-purpose.md + 05-rules.md.
             progress.md: last 1–2 entries only. git log --oneline -15
2. AUDIT   → Read all relevant files, trace data flow end-to-end
3. PLAN    → Write a short plan: which files, what changes, what tests
4. BRANCH  → Work on `main` (active branch, v3.7.0 merge complete)
5. CODE    → Implement changes inside LUCX-HOOK blocks in upstream files;
             new code goes in internal/awg/ or internal/lucx/
6. TEST    → Run tests:
               go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v
               cd frontend && npm run typecheck && npm run lint
7. BUILD   → Frontend: cd frontend && npm run build
             Backend:  go build -o /tmp/x-ui .
                       (requires frontend/dist to exist for //go:embed)
8. DEPLOY  → SCP to vps_finland_lucx, restart x-ui.service
9. VERIFY  → Check `sudo systemctl status x-ui`, check server logs
10. COMMIT → `git add` specific files, `git commit` with descriptive message (Russian)
11. STATUS → Output `git status` and `git log --oneline -15` after commits
11.5. CHECK PR/ISSUES → BEFORE every push ALWAYS check open PRs and issues:
             `gh pr list --repo AlexeyLCP/lucx-ui --state open`
             `gh issue list --repo AlexeyLCP/lucx-ui --state open`
             If there are unhandled PRs (not yours) or issues — do NOT push
             immediately. Tell the user: which PRs/issues are open, by whom, and
             offer: (a) review/merge the PR first, (b) fix the issue first,
             (c) push after. Do not silently push over someone else's PR —
             you can overwrite or break their work.
12. DOCS   → ALWAYS keep progress.md and .agents/ up to date. Every commit —
             a new entry in progress.md (what was done, which lucxVersion, which
             files, which tests). On architecture / rules / env / debug changes —
             update the matching .agents/ file. Do not grow AGENTS.md.
             Write all agent/project docs in English (Rule 11).
```

## Test Commands

```bash
# Go unit tests (no cgo required)
go test ./internal/awg/... ./internal/lucx/... ./internal/database/... -count=1 -v

# Frontend
cd frontend && npm run typecheck && npm run lint

# Full project build (requires frontend/dist) — LINUX/CGO ONLY.
# On Windows without gcc this `go build` fails in internal/database (sqlite3.Backup,
# CGO) — that is PRE-existing, not a regression, and NOT a gate. The full binary and
# all tests are authoritatively built by GitHub Actions (CI + release.yml, ubuntu, CGO).
# Locally gate on gofumpt + targeted no-cgo `go test` + frontend checks;
# for a full build/tests — push to main → watch `gh run list` / CI.
cd frontend && npm run build && cd ..
go build -o /tmp/x-ui .

# Pre-push hygiene (gofumpt on all LucX files — catches Windows/Linux drift before CI)
bin/check-lucx.sh          # check;  bin/check-lucx.sh -w  # autofix

# CRITICAL before push (CI catches these, check-lucx.sh does NOT):
#   1. gofumpt on the WHOLE repo (CI's golangci-lint runs on ./..., not just
#      the 49 LucX files). check-lucx.sh only covers LucX-owned files, so a
#      pre-existing formatting drift in an upstream file you touched (e.g. a
#      case-block indentation) will pass locally and fail CI.
gofumpt -l .               # list;  gofumpt -w <file>  # fix one
#   2. Regenerate OpenAPI artifacts after editing any Go struct with json:/example:
#      tags that flows into an API response. CI's `codegen` job fails on stale
#      generated files. AGENTS.md Rule 9 "Do not edit src/generated/" means no
#      HAND edits — regenerating via the tool is required and expected.
cd frontend && npm run gen  # gen:zod (go run ./tools/openapigen) + gen:api

# Optional: install the git hook that runs check-lucx + fast tests + PR/issues guard (step 11.5)
cp bin/pre-push .git/hooks/pre-push && chmod +x .git/hooks/pre-push
```

---

## Release & Install (fork)

`install.sh` is adapted for our fork (`AlexeyLCP/lucx-ui`): downloads the release tarball and raw scripts (x-ui.sh, x-ui.rc, service units) from `main`. Xray-core + mtg are reused from the upstream `MHSanaei/3x-ui` release.

GitHub is the source of truth. SourceCraft (`sc`, `alexeylcp/lucx-ui`) builds and tags independently (`.sourcecraft/ci.yaml` + `bin/sourcecraft-release.sh`). Optional install from Yandex: `install.sh --yandex` (or `LUCX_SOURCE=yandex`). The choice is stored in `/etc/x-ui/install-source` so later `x-ui update` stays on that host.

### Release build — GitHub Actions only (do NOT build on a VPS by hand)

All builds are done by `.github/workflows/release.yml` (ubuntu-latest, CGO via
Bootlin musl-toolchain → static binary, Node from `.nvmrc` (=24, Vite 8
won’t build on Node 20). Manual VPS builds (`bin/build-release.sh`) are
legacy — don’t use them: they drift from CI on xray/mtg versions.

```bash
# 0. REQUIRED before tagging — unreleased commits (lesson lucx.133):
#    the tag pins the WHOLE main, but notes easily describe only the last fix
#    and lose features/fixes that landed on main without their own tag.
git fetch gh --tags
git log --oneline v3.7.0-lucx.$((N-1))..HEAD
# In notes and progress.md — EVERY non-docs commit from that list, not only lucx.N.
# Empty list except the previous release’s docs tail — OK.

# 1. Wait for green CI on main, then tag — Release workflow
#    will build the tarball and publish the stable release:
git tag v3.7.0-lucx.N && git push gh v3.7.0-lucx.N
gh run watch --repo AlexeyLCP/lucx-ui          # Release LucX-UI
gh release view v3.7.0-lucx.N --repo AlexeyLCP/lucx-ui   # asset x-ui-linux-amd64.tar.gz
# Same tag also runs docker.yml → ghcr.io/alexeylcp/lucx-ui:<tag> and :latest
# (amd64+arm64). First publish: GitHub → Packages → lucx-ui → Public.

# 2. REQUIRED: release body (release notes) — what the operator sees
#    in the panel on update (getPanelUpdateInfo → releaseNotes).
#    upload-release-action often leaves body empty → fill it immediately:
gh release edit v3.7.0-lucx.N --repo AlexeyLCP/lucx-ui --notes-file - <<'EOF'
## v3.7.0-lucx.N

- item 1 (what changed for the user)
- item 2
- …
EOF
# Source: lucx.N entry in progress.md (compress to 5–15 bullets, in Russian
# or EN+RU). Without notes the operator sees empty “What’s new” or raw compare.

# 3. Install/update the panel on a VPS:
bash <(curl -fL https://raw.githubusercontent.com/AlexeyLCP/lucx-ui/main/install.sh)
# or on an already installed one: x-ui update (console) / button in the web panel
```

### Release notes — law (mandatory)

**Every stable tag `v3.7.0-lucx.N` MUST have a non-empty GitHub Release body.**

- The panel shows `release.Body` in the update modal (`PanelUpdateInfo.releaseNotes`).
- Empty body → fallback `fetchCompareNotes` (raw commit subjects) — bad for testers.
- Write **after** a successful Release workflow (`gh release edit … --notes` / `--notes-file`).
- In parallel: entry in `progress.md` (detailed, for agents) + notes (short for UI/testers).
- If notes were forgotten on an already published tag — fill them immediately (`gh release edit`), don’t wait for the next release.

#### Notes style (how we write for people)

**Language: RU + EN.** First a Russian block, then `---` and the same meaning in English (shorter is fine, same meaning). Panel and GitHub are read by both RU and EN operators.

Tone: casual, admin-to-their-people. No corporate speak, bureaucracy, or filler. Short, to the point, light humor OK. Technical stuff simple, no lectures. EN — same tone (casual), not “We are pleased to announce”.

Formatting:
- Emoji heading at the top (`🔈` / `🆕` / `⚠️` / `🔧` / `✅` etc.)
- Short paragraphs, lots of air
- Lists with `1️⃣2️⃣3️⃣` or bullets `🔘` `✅` `❎`
- `---` separators between blocks (and between RU/EN)
- Important bits — **bold**
- Commands / paths / protocols — in `` `code` ``
- End RU with: `⚡️ Приятного использования!` · EN: `⚡️ Enjoy!`

Structure (each language):
1. What happened + why (1–2 sentences)
2. What changed (list)
3. What the user should do (if needed)
4. Short closer

Forbidden: long intros; “Мы рады сообщить” / “We are pleased to announce”; wall of text without emoji; over-formality; dump of diff/commit hash; RU-only or EN-only.

**Before tagging the agent must:** `git log --oneline <last-stable-tag>..HEAD` and include in notes all our unreleased changes (not only the current commit’s topic). Otherwise a commit like lucx.132+ (outbound placeholder) “gets lost” — it won’t appear in the panel’s “What’s new”. Docs-only tail of the previous release need not be duplicated.

The tag is set **only after green CI on main** (lesson lucx.48). `lucxVersion`
in `internal/config/config.go` must match the tag suffix — the guard in
release.yml fails the build on mismatch. Push to main without a tag updates
the rolling pre-release `dev-latest` (panel Dev channel); `releases/latest`
stays on the last stable tag.

### What `x-ui update` does (lucx.58+)
1. Installs the new binary/frontend, stops the panel.
2. **Auto kernel upgrade** to the latest packaged (Debian/Ubuntu meta-package) — only if AWG is already installed (inside `install-awg-module.sh`).
3. AWG-gate (only if the module was already installed: marker / `amneziawg` loaded / `awg-quick` in PATH). Else skip — `x-ui install-awg`. Marker `/etc/x-ui/.awg-module-version` vs `git ls-remote refs/heads/master`; mismatch → `--force-rebuild`.
4. Start panel, migrate, fail2ban; if a new kernel was installed — **reboot in 10s** (AWG module already built for the new kernel; panel comes up via systemd).

### VPS build dependencies
- Go 1.23+ (1.26 recommended)
- Node.js 20+ and npm
- gcc (for CGO)
- git, curl, tar

### Release layout (same as upstream)
```
x-ui-linux-amd64.tar.gz → x-ui/
  ├── x-ui                    ← our binary (CGO, built from the fork)
  ├── x-ui.sh, x-ui.rc        ← from the repo
  ├── x-ui.service.{debian,arch,rhel}  ← from the repo
  └── bin/
      ├── xray-linux-amd64    ← from upstream release (not our code)
      ├── mtg-linux-amd64     ← from upstream release (not our code)
      ├── install-awg-module.sh  ← our DKMS script
      └── caddy-naive / olcrtc / qwdtt / mieru / trusttunnel / anytls / tproxy / mtproxy (+ clients)
          ← GitHub amd64 tarball unpacks gz from third_party/sidecars/; tproxy+mtproxy are built in release.yml
```

Geo is not in the panel tarball (GitHub slim / SourceCraft 100 MB). `install.sh` / `update.sh` fetch Loyalsoldier + IR/RU/ROSCOM **before** panel start (never fatal). SourceCraft unpacks `x-ui-geo.tar.gz` from the dist bundle at the same point.

Tunnel sidecars (gzipped) live in `third_party/sidecars/linux-amd64/`. GitHub amd64 tarball includes the unpacked binaries (lucx.184 — first install was missing cores). SourceCraft stays SLIM (100 MB cap); `install.sh` / `update.sh` still fetch after start as a refresh.

---

## Branch Protection (gh/main)

`main` on `AlexeyLCP/lucx-ui` is protected (Settings → Branches): **force-push and branch deletion are forbidden for everyone, including admin** (`enforce_admins: true`, `allow_force_pushes: false`, `allow_deletions: false`). PRs and status checks are NOT required — direct pushes work as before. If force-push is ever needed (e.g. history squash) — consciously relax the rule in Settings → Branches first, do it, put the rule back. Two-step by design.

---

## Commit Convention

- Prefixes: `feat:`, `fix:`, `refactor:`, `chore:`, `docs:`, `test:`
- Scope: `feat(awg): ...`, `fix(frontend): ...`, `chore(codegen): ...`
- Commit messages — in Russian (unless requested otherwise)
- Example: `feat(awg): порт изолированных пакетов на v3.5.0`

## Frontend Conventions

- Ant Design 6 only — no Tailwind/shadcn.
- TS strict; `@typescript-eslint/no-explicit-any` is an error. Zod schemas in `src/schemas/` are the source of truth; infer types with `z.infer`, never hand-write. Do not edit `src/generated/`.
- Editing `frontend/src` does NOT change what users see until the Vite build is regenerated into `internal/web/dist/`.
- After touching share-link logic (`src/lib/xray/`), run `npm run test` (golden fixtures).

---

## Go Conventions

- Stdlib `testing` only (no testify). Table-driven, `t.Run` subtests.
- NO `//` line comments in committed Go/TS (except directives like `//go:build`). Names carry meaning. (Inherited from upstream CLAUDE.md — applies to upstream code; LucX HOOK blocks may carry the `// LUCX-HOOK:` marker comment by design.)
- `golangci-lint run` / `make lint` for formatting (gofumpt + goimports).
- Conventional-commit prefixes, Russian commit messages.

---
