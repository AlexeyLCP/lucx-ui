// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package cps

import "fmt"

// sipSession is one registration: the unauthenticated REGISTER, the retry
// carrying Digest credentials, an OPTIONS ping, then RFC 5626 keepalives. Five
// independent REGISTERs down one socket was never a client's behaviour.
func sipSession(domain string) [5]Descriptor {
	if domain == "" {
		domain = PickRandomDomain(sipDomains)
	}
	// Everything that identifies the dialog is literal: a hole here would be
	// redrawn per packet and I1, I2 and I3 would stop belonging together.
	user := randLowerAlphaNum(rng.Intn(5) + 4)
	callID := randomHex(8)
	tag := randomHex(4)
	privIP := randomPrivateIP()
	port := []int{5060, 5062, 5080, 5160}[rng.Intn(4)]
	cseq := rng.Intn(50) + 1

	common := func(method string, seq int, extra string) Descriptor {
		head := fmt.Sprintf("%s sip:%s SIP/2.0\r\nVia: SIP/2.0/UDP %s:%d;branch=z9hG4bK", method, domain, privIP, port)
		tail := fmt.Sprintf("\r\nFrom: <sip:%s@%s>;tag=%s\r\nTo: <sip:%s@%s>\r\nCall-ID: %s@%s\r\nCSeq: %d %s\r\n%sMax-Forwards: 70\r\nUser-Agent: Linphone/5.1.2 (belle-sip/1.6.3)\r\nContent-Length: 0\r\n\r\n",
			user, domain, tag, user, domain, callID, privIP, seq, method, extra)
		// The branch is a per-transaction token, so it is the one field that
		// should differ on every send.
		return Descriptor{}.Lit([]byte(head)).RandChars(7).RandDigits(7).Lit([]byte(tail))
	}

	digest := fmt.Sprintf("Authorization: Digest username=\"%s\", realm=\"%s\", nonce=\"%s\", uri=\"sip:%s\", response=\"%s\", algorithm=MD5\r\n",
		user, domain, randomHex(16), domain, randomHex(16))
	contact := fmt.Sprintf("Contact: <sip:%s@%s:%d>;expires=3600\r\nExpires: 3600\r\n", user, privIP, port)

	return [5]Descriptor{
		common("REGISTER", cseq, contact),
		common("REGISTER", cseq+1, contact+digest),
		common("OPTIONS", cseq+2, ""),
		sipKeepalive(),
		sipKeepalive(),
	}
}

// sipKeepalive is the four-byte double CRLF of RFC 5626 §3.5.1.
func sipKeepalive() Descriptor {
	return Descriptor{}.Lit([]byte("\r\n\r\n"))
}
