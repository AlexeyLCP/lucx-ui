// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package tunnel

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mhsanaei/3x-ui/v3/internal/config"
)

// defaultAcmeHTTPChallengePort mirrors LUCX_ACME_DEFAULT_PORT in bin/lucx-acme.sh.
const defaultAcmeHTTPChallengePort = 29380

// acmePortFile is written by the x-ui.sh / install.sh ACME prompt (bin/lucx-acme.sh).
var acmePortFile = func() string {
	return filepath.Join(config.GetDBFolderPath(), "acme-http-port")
}

// AcmeHTTPChallengePort is where acme.sh standalone listens on loopback. Port 80
// is never valid: LucX forwards :80 to this port.
func AcmeHTTPChallengePort() int {
	raw, err := os.ReadFile(acmePortFile())
	if err != nil {
		return defaultAcmeHTTPChallengePort
	}
	p, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil || p < 1 || p > 65535 || p == 80 {
		return defaultAcmeHTTPChallengePort
	}
	return p
}

// writeAcmeChallengeHandle must precede every other handler of a :80 site:
// Caddy's own HTTP-01 tokens are answered earlier, unknown ones reach acme.sh.
func writeAcmeChallengeHandle(b *strings.Builder, indent string) {
	b.WriteString(indent + "handle /.well-known/acme-challenge/* {\n")
	b.WriteString(indent + "\treverse_proxy 127.0.0.1:" + strconv.Itoa(AcmeHTTPChallengePort()) + "\n")
	b.WriteString(indent + "}\n")
}

// writeHTTPRedirectSite is the :80 block. Both arms are handle blocks because a
// bare redir sorts before handle in Caddy's directive order and would win.
func writeHTTPRedirectSite(b *strings.Builder, address, bind string) {
	b.WriteString(address + " {\n")
	if bind != "" {
		b.WriteString("\tbind " + bind + "\n")
	}
	writeAcmeChallengeHandle(b, "\t")
	b.WriteString("\thandle {\n\t\tredir https://{host}{uri} permanent\n\t}\n}\n")
}
