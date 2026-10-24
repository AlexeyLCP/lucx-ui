// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"errors"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	kernelwg "github.com/mhsanaei/3x-ui/v3/internal/lucx/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
	"github.com/op/go-logging"
)

type kernelJobStore struct {
	rows         []*model.Inbound
	fail         bool
	saved        []int64
	emails, tags []string
}

func (s *kernelJobStore) GetAllInbounds() ([]*model.Inbound, error) { return s.rows, nil }
func (s *kernelJobStore) AddTraffic(_ []*xray.Traffic, c []*xray.ClientTraffic) (bool, bool, error) {
	var n int64
	for _, r := range c {
		n += r.Up
	}
	s.saved = append(s.saved, n)
	if s.fail {
		s.fail = false
		return false, false, errors.New("temporary storage failure")
	}
	return false, false, nil
}
func (s *kernelJobStore) RefreshLocalOnlineClients(e, t []string) { s.emails = e; s.tags = t }

type kernelJobManager struct {
	usage     []kernelwg.Usage
	preserved []int
	revoked   []int
}

func (m *kernelJobManager) ReconcilePreserving(_ []kernelwg.Instance, p []int) []error {
	m.preserved = p
	return nil
}

func (m *kernelJobManager) RevokeDisallowed(ib *model.Inbound) error {
	m.revoked = append(m.revoked, ib.Id)
	return nil
}
func (m *kernelJobManager) CollectTraffic() []kernelwg.Usage { return m.usage }
func TestKernelWireguardJobRetriesPersistenceWithoutInflatingSpeed(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	now := time.Now()
	store := &kernelJobStore{fail: true}
	manager := &kernelJobManager{usage: []kernelwg.Usage{{Delta: kernelwg.Delta{Up: 100, Online: true}, Email: "peer", Tag: "active"}}}
	j := &KernelWireguardJob{store: store, manager: manager, now: func() time.Time { return now }}
	j.Run()
	if len(j.pending) != 1 || len(store.tags) != 1 || store.tags[0] != "active" {
		t.Fatal("failed write lost pending traffic or online status")
	}
	now = now.Add(10 * time.Second)
	manager.usage = []kernelwg.Usage{{Delta: kernelwg.Delta{Up: 50}, Email: "peer", Tag: "active"}}
	j.Run()
	if len(j.pending) != 0 || len(store.saved) != 2 || store.saved[1] != 150 {
		t.Fatalf("pending retry was lost or duplicated: %+v", store.saved)
	}
	if len(store.tags) != 0 || len(store.emails) != 0 {
		t.Fatal("offline peers published online tags")
	}
	kernelWGSpeed.Lock()
	speed := kernelWGSpeed.snapshot.clients
	kernelWGSpeed.Unlock()
	if len(speed) != 1 || speed[0].Up != 25 {
		t.Fatalf("retry bytes inflated fresh speed: %+v", speed)
	}
}

func TestKernelWireguardJobPreservesMalformedInbound(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	store := &kernelJobStore{rows: []*model.Inbound{{Id: 7, Protocol: model.WireGuard, Enable: true, Settings: `{"kernel":true,"secretKey":"invalid"}`}}}
	manager := &kernelJobManager{}
	j := &KernelWireguardJob{store: store, manager: manager, now: time.Now}
	j.Run()
	if len(manager.revoked) != 1 || manager.revoked[0] != 7 {
		t.Fatal("invalid inbound skipped revocation")
	}
	if len(manager.preserved) != 1 || manager.preserved[0] != 7 {
		t.Fatal("invalid inbound was not preserved")
	}
}
