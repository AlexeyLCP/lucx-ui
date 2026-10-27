# Security Policy

This repository is a fork of [MHSanaei/3x-ui](https://github.com/MHSanaei/3x-ui).
Where the problem lives decides who must hear about it.

## LucX-UI code (this fork)

LucX-specific code is `internal/awg/`, `internal/lucx/`, `LUCX-HOOK` blocks
inside otherwise-upstream files (see `.agents/08-hooks.md` for the list), and
the LucX shell scripts under `bin/`. Problems in that code are handled here,
not upstream:

1. **Preferred:** GitHub Private vulnerability reporting — the **Security**
   tab of this repository → **Report a vulnerability**. Private reporting is
   enabled for this repo; the LucX maintainer replies in the advisory thread.
2. If you cannot use GitHub's private reporting, contact the maintainer
   (`AlexeyLCP`) through the project's established support channels. Include
   the affected `lucxVersion` (see `internal/config/config.go`), reproduction
   steps, and the impact you see.

Do not open a public issue for anything you believe is exploitable — an
authentication bypass, remote code execution, injection, secret or credential
exposure, privilege escalation. A public report gives attackers a head start.

There is no bug-bounty program for the fork. Fixes ship in the next
`lucx.*` release, and the advisory is published after a fixed version is
available.

## Vanilla 3x-ui code (upstream)

If the vulnerability is in vanilla 3x-ui code (Xray integration, panel core,
upstream frontend — anything outside the LucX-specific list above), report it
to upstream, which owns its fixes: use the
[3x-ui Security tab](https://github.com/MHSanaei/3x-ui/security) or the
upstream security contact, ho3ein.sanaei@gmail.com. Automated reports from
Anthropic's OSS Scanner that are not LucX-specific also go to that address.

There is no bug-bounty program. Fixes ship in the next release, and the
advisory is published after a fixed version is available.

## Supported versions

Only the latest release receives security fixes. Update with the install
script or your package channel and confirm the problem still exists before
reporting.
