# 07 — Debug upstream sync

Extracted from AGENTS.md. This file is project law.

---

### Pattern 2: LUCX-HOOK conflict on upstream sync
- **Cause:** Upstream changed a file with a HOOK marker between releases.
- **Fix:** Resolve each block separately (see Rule 8). Don’t `git checkout` the whole file and don’t blanket `--ours` — you’ll lose upstream changes.

### Pattern 2b: after merge a file lost all LUCX-HOOK blocks
- **Cause:** the file was edited in conflict state via the IDE — it rewrites from its merge cache and silently drops content. On v3.6.0: `install.sh` lost all 16 blocks, `db.go` — upstream functions.
- **Detect:** `git grep -c "LUCX-HOOK"` before and after — a drop in count is a loss. Also check line count: if the result is smaller than BOTH ours and upstream — code fell out.
- **Fix:** `git checkout --merge -- <files>` restores conflict state with markers; then resolve only from the terminal, then `git add` to clear unmerged entries (else the IDE keeps offering its master and can overwrite the work).

### Pattern 2c: CI red on i18n-dead-keys after adding a LucX feature
- **Cause:** `frontend/src/test/i18n-dead-keys.test.ts` (from v3.6.0) requires **every** one of the 13 locales to carry exactly the same key set as `en-US.json` (and vice versa — no extras). Added a key only in en+ru — 11 locales fail with `missing=N`.
- **Fix:** add new keys to all 13 files `internal/web/translation/*.json` at once, with a translation into the locale language (project convention); technical terms (`Tag`, `MTU`, `DNS`, `Allowed IPs`, `awg-quick`, `outbound`) stay Latin. The second suite test requires the reverse: an unused-in-code key is also a failure — delete it from all 13.
