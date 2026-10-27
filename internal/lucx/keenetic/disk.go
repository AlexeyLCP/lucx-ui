// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package keenetic holds Entware-only probes. A normal Linux server never
// has /opt/bin/opkg, so these stay off the hot path there.
package keenetic

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/disk"
)

const cacheTTL = 30 * time.Second

var (
	rciBase = "http://127.0.0.1:79/rci"

	cacheMu               sync.Mutex
	cacheAt               time.Time
	cacheUsed, cacheTotal uint64
	cacheOK               bool
)

// OPKGUsage is the volume Entware is installed on. Keenetic's statfs("/")
// is the firmware RAM overlay, so the dashboard would show a few dozen MB
// instead of the USB stick or the internal ROM partition. ok is false when
// this host is not Entware.
func OPKGUsage() (used, total uint64, ok bool) {
	if !entware() {
		return 0, 0, false
	}
	cacheMu.Lock()
	if !cacheAt.IsZero() && time.Since(cacheAt) < cacheTTL {
		used, total, ok = cacheUsed, cacheTotal, cacheOK
		cacheMu.Unlock()
		return used, total, ok
	}
	cacheMu.Unlock()

	used, total, ok = usageFromRCI(rciGet)
	if !ok {
		if info, err := disk.Usage("/opt"); err == nil && info.Total > 0 {
			used, total, ok = info.Used, info.Total, true
		}
	}
	cacheMu.Lock()
	cacheAt = time.Now()
	cacheUsed, cacheTotal, cacheOK = used, total, ok
	cacheMu.Unlock()
	return used, total, ok
}

func entware() bool {
	st, err := os.Stat("/opt/bin/opkg")
	return err == nil && !st.IsDir()
}

func rciGet(path string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rciBase+path, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, errStatus
	}
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

var errStatus = errString("keenetic rci: bad status")

type errString string

func (e errString) Error() string { return string(e) }

func usageFromRCI(get func(path string) ([]byte, error)) (uint64, uint64, bool) {
	diskBody, err := get("/show/sc/opkg/disk")
	if err != nil {
		return 0, 0, false
	}
	lsBody, err := get("/ls")
	if err != nil {
		return 0, 0, false
	}
	return parseOPKGDisk(diskBody, lsBody)
}

func parseOPKGDisk(diskBody, lsBody []byte) (used, total uint64, ok bool) {
	var disk struct {
		Disk string `json:"disk"`
	}
	if json.Unmarshal(diskBody, &disk) != nil {
		return 0, 0, false
	}
	label := trimLabel(disk.Disk)
	if label == "" {
		return 0, 0, false
	}
	var ls map[string]json.RawMessage
	if json.Unmarshal(lsBody, &ls) != nil {
		return 0, 0, false
	}
	if raw, exists := ls["entry"]; exists {
		var entry map[string]json.RawMessage
		if json.Unmarshal(raw, &entry) == nil {
			for _, node := range entry {
				if u, t, ok := matchNode(node, label); ok {
					return u, t, true
				}
			}
		}
	}
	for key, node := range ls {
		if key == "entry" {
			continue
		}
		if trimLabel(key) == label {
			if u, t, ok := pairFrom(node); ok {
				return u, t, true
			}
		}
		if u, t, ok := matchNode(node, label); ok {
			return u, t, true
		}
	}
	return 0, 0, false
}

func matchNode(raw json.RawMessage, label string) (uint64, uint64, bool) {
	var node struct {
		Label string `json:"label"`
	}
	if json.Unmarshal(raw, &node) != nil || trimLabel(node.Label) != label {
		return 0, 0, false
	}
	return pairFrom(raw)
}

func pairFrom(raw json.RawMessage) (uint64, uint64, bool) {
	var node struct {
		Free  int64 `json:"free"`
		Total int64 `json:"total"`
	}
	if json.Unmarshal(raw, &node) != nil || node.Total <= 0 || node.Free < 0 || node.Free > node.Total {
		return 0, 0, false
	}
	return uint64(node.Total - node.Free), uint64(node.Total), true
}

func trimLabel(s string) string {
	return strings.Trim(strings.TrimSpace(s), ":/")
}
