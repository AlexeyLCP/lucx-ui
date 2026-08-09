// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package vpnuri encodes AmneziaVPN vpn:// share links from a WireGuard/AWG
// client .conf. Format matches amnezia-client ImportController:
//
//	vpn:// + Base64URL( qCompress(payload) )
//
// where qCompress is Qt's wrapper: 4-byte big-endian uncompressed length +
// raw DEFLATE (zlib) body. Amnezia accepts both plain JSON containers and
// compressed conf text after qUncompress; we ship the .conf text so the
// client's WireGuard/Awg conf parser (extractWireGuardConfig) runs.
package vpnuri

import (
	"bytes"
	"compress/zlib"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"strings"
)

// EncodeConf builds a vpn:// URI from an awg-quick client .conf body.
func EncodeConf(conf string) (string, error) {
	conf = strings.TrimSpace(conf)
	if conf == "" {
		return "", fmt.Errorf("vpnuri: empty conf")
	}
	if !strings.Contains(conf, "[Interface]") || !strings.Contains(conf, "[Peer]") {
		return "", fmt.Errorf("vpnuri: conf missing [Interface]/[Peer]")
	}
	payload := []byte(conf)
	compressed, err := qCompress(payload)
	if err != nil {
		return "", err
	}
	b64 := base64.RawURLEncoding.EncodeToString(compressed)
	return "vpn://" + b64, nil
}

// qCompress mirrors Qt qCompress: big-endian uint32 uncompressed size + zlib.
func qCompress(data []byte) ([]byte, error) {
	var buf bytes.Buffer
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(data)))
	buf.Write(size[:])
	w := zlib.NewWriter(&buf)
	if _, err := w.Write(data); err != nil {
		_ = w.Close()
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Decode conf back (tests / diagnostics). Returns uncompressed payload bytes.
func Decode(uri string) ([]byte, error) {
	s := strings.TrimSpace(uri)
	s = strings.TrimPrefix(s, "vpn://")
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		// try padded
		raw, err = base64.URLEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}
	}
	if len(raw) < 4 {
		return nil, fmt.Errorf("vpnuri: truncated payload")
	}
	// skip 4-byte size header (Qt)
	r, err := zlib.NewReader(bytes.NewReader(raw[4:]))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	var out bytes.Buffer
	if _, err := out.ReadFrom(r); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}
