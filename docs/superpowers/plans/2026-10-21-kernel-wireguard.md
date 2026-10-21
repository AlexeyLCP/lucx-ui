# Kernel WireGuard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Manage standard Linux kernel WireGuard peers through LucX with Xray TPROXY, validate on the operator's gateway, and submit an upstream PR.

**Architecture:** Explicit optional kernel backend on existing WireGuard inbounds. Isolated runtime in internal/lucx/wireguard with panel integration through existing runtime, traffic and configuration paths.

**Tech Stack:** Go 1.27, React/TypeScript/Zod, Linux wireguard-tools, nftables, SQLite, GitHub Actions.

**Spec:** docs/superpowers/specs/2026-10-21-kernel-wireguard-design.md

## Global Constraints
- No issued client keys/addresses/settings change without explicit operator action.
- Existing wireguard inbounds default to the existing Xray implementation.
- Runtime is Linux-only; IPv4 TPROXY only in the first version, explicit errors otherwise.
- No imported shell hooks, secrets in logs, builds on production hosts or unrelated rewrites.
- LUCX-HOOK upstream integration; new LucX files preserve repository licensing conventions.

## Review Focus
- Peer-only edits keep existing handshakes and do not restart Xray.
- Disable/expiry/quota removal converges without reconnecting other peers.
- UDP mark restore chains cannot bypass TPROXY.
- Import preserves issued parameters and never executes hooks or adopts foreign interfaces implicitly.
- Fresh and legacy databases retain existing settings, keys and subscription output.

### Task 1: Kernel state, validation and rendering
Files: internal/lucx/wireguard/{instance,config,manager,traffic,import}.go and matching tests.
Interfaces: IsKernel(*model.Inbound) bool; Parse(*model.Inbound) (Instance,error); Manager.Ensure/Remove/Reconcile/CollectTraffic/StopAll.
- [x] Write failing tests for opt-in parsing, invalid keys/subnets/IPv6, disabled peers, standard-only config, peer-independent fingerprint, counters and cleanup ownership.
- [x] Run the new tests and record expected failures.
- [x] Implement ip/wg lifecycle, syncconf, owned config files, scoped nft TPROXY and route cleanup.
- [x] Verify unit tests and Linux namespace integration including unrelated-interface preservation.

### Task 2: Panel runtime, routing and accounting
Files: internal/web/{runtime/local.go,service/xray.go,service/inbound.go,web.go}; new LucX-owned service/job files; regression tests.
Interfaces: existing inbound/client endpoints remain unchanged; new job translates kernel deltas to existing traffic/speed feed.
- [x] Write failing config tests: kernel opt-in replaced with TPROXY, no kernel=false changes, original route order retained; runtime peer-only update does not call Xray API.
- [x] Implement opt-in validation and add/update/delete/disable handling; reconcile startup, traffic/online and shutdown.
- [x] Test database overlay, quotas/expiry, stopped/recovered interface and accounting without duplicate inbound totals.

### Task 3: UI, export and explicit import
Files: frontend/src/schemas/protocols/inbound/wireguard.ts; frontend/src/pages/inbounds/form/protocols/wireguard.tsx; LucX-owned import service/controller; API docs; translation files; tests.
Interfaces: optional kernel/address/tproxyPort settings; authenticated import preview endpoint returns settings and peer metadata only after explicit request.
- [x] Write failing schema/export/import tests for default Xray, kernel settings persistence, unchanged standard client exports and rejection of imported hooks.
- [x] Add backend selector, address and TPROXY port fields with IPv4 scope/help; retain ordinary WireGuard QR and downloads.
- [x] Implement config preview/import without starting/stopping foreign interfaces; document missing client private keys.
- [x] Run frontend unit/components tests, typecheck/lint, codegen and build; Go service/controller tests.

### Task 4: Review, fork CI and production verification
Files: deployment runbook/test scripts and docs; dedicated fork build workflow only if existing workflow cannot build the feature ref.
- [x] Run Go tests/build/vet, required LucX checks, frontend tests/typecheck/lint/build; independent review and repair findings.
- [x] Inspect upstream open PRs/issues for conflicts; push only the personal feature branch, build Linux CI artifacts.
- [x] Back up live installation and schedule rollback; deploy artifact, synthetic WG inbound smoke test and multiple rekeys.
- [x] Assess existing wg0 ownership before transfer; preserve external ownership when the separate provisioning service still owns it. Verify WG/AWG/transit/accounting, remove test resources, and cancel rollback only after passing.
- [x] Record limits and evidence; submit upstream PR, record its URL and report final status.

## Status

Proposed in [PR #147](https://github.com/AlexeyLCP/lucx-ui/pull/147). Runtime validation evidence is kept in that PR and the operator KB. External wg0 provisioning ownership was deliberately preserved.
