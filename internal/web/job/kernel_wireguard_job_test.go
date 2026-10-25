// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package job

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/op/go-logging"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/logger"
	"github.com/mhsanaei/3x-ui/v3/internal/lucx/tunnel"
	kernelwg "github.com/mhsanaei/3x-ui/v3/internal/lucx/wireguard"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
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

type kernelJobWgo struct{ reconciled []tunnel.Instance }

func (w *kernelJobWgo) ReconcileWgo(want []tunnel.Instance) { w.reconciled = want }

func obfsKernelInbound(id int) *model.Inbound {
	return &model.Inbound{Id: id, Port: 51820, Protocol: model.WireGuard, Enable: true, Settings: `{"kernel":true,"secretKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","address":"10.8.0.1/24","tproxyPort":40000,"mtu":1420,
"obfuscation":{"enabled":true,"blockDirectPort":false,"presets":[
 {"id":"p1","name":"phobos-ru","extPort":51822,"key":"k1","masking":"STUN",
  "maxDummy":4,"idleTimeout":300,"clientWgLocalPort":13255,"isDefault":true}
]}}`}
}

func TestKernelWireguardJobReconcilesWgo(t *testing.T) {
	store := &kernelJobStore{rows: []*model.Inbound{obfsKernelInbound(3)}}
	manager := &kernelJobManager{}
	wgo := &kernelJobWgo{}
	var fwStates []kernelwg.WgoFirewallState
	restore := syncWgoFirewall
	syncWgoFirewall = func(s []kernelwg.WgoFirewallState) { fwStates = s }
	defer func() { syncWgoFirewall = restore }()

	j := &KernelWireguardJob{store: store, manager: manager, wgo: wgo, now: time.Now}
	j.Run()

	if len(wgo.reconciled) != 1 || wgo.reconciled[0].Key != "wgo-3" || !wgo.reconciled[0].Enabled {
		t.Fatalf("wgo reconcile = %+v", wgo.reconciled)
	}
	if len(fwStates) != 1 || fwStates[0].ID != 3 || fwStates[0].WGPort != 51820 {
		t.Fatalf("fw states = %+v", fwStates)
	}

	// plain kernel inbound → no wgo instance at all
	plain := obfsKernelInbound(4)
	plain.Settings = `{"kernel":true,"secretKey":"AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=","address":"10.8.0.1/24","tproxyPort":40001,"mtu":1420}`
	store.rows = []*model.Inbound{plain}
	wgo.reconciled = nil
	j.Run()
	if len(wgo.reconciled) != 0 {
		t.Fatalf("plain inbound must not produce wgo: %+v", wgo.reconciled)
	}

	// disabled obfuscation block → instance present but disabled (stops the process)
	off := obfsKernelInbound(5)
	off.Settings = strings.Replace(off.Settings, `"enabled":true`, `"enabled":false`, 1)
	store.rows = []*model.Inbound{off}
	wgo.reconciled = nil
	j.Run()
	if len(wgo.reconciled) != 1 || wgo.reconciled[0].Enabled {
		t.Fatalf("disabled block must yield stopped instance: %+v", wgo.reconciled)
	}
}

func TestKernelWireguardJobDedupesRepeatingReconcileError(t *testing.T) {
	logger.InitLogger(logging.ERROR)
	j := &KernelWireguardJob{now: time.Now}
	same := errors.New("kernel WireGuard inbound 14: wg missing")
	if n := j.warnReconcile([]error{same}); n != 1 {
		t.Fatalf("first occurrence logged %d lines", n)
	}
	if n := j.warnReconcile([]error{same}); n != 0 {
		t.Fatalf("identical repeat logged %d more lines", n)
	}
	other := errors.New("kernel WireGuard inbound 15: port busy")
	if n := j.warnReconcile([]error{same, other}); n != 1 {
		t.Fatalf("new message logged %d lines", n)
	}
	if n := j.warnReconcile(nil); n != 0 {
		t.Fatalf("clean tick logged %d lines", n)
	}
	if j.reconcileWarned != nil {
		t.Fatal("clean tick did not clear dedupe state")
	}
	if n := j.warnReconcile([]error{same}); n != 1 {
		t.Fatalf("post-clean repeat logged %d lines", n)
	}
}
