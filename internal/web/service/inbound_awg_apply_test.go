package service

import (
	"context"
	"sync"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/web/runtime"
)

// recordingRuntime keeps the arguments, not just the counts: the old snapshot's
// protocol is what decides which sidecar a teardown reaches.
type recordingRuntime struct {
	fakeNodeRuntime
	mu   sync.Mutex
	call []dispatchCall
}

type dispatchCall struct {
	op       string
	oldProto model.Protocol
	newProto model.Protocol
}

func (r *recordingRuntime) record(c dispatchCall) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.call = append(r.call, c)
}

func (r *recordingRuntime) calls() []dispatchCall {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]dispatchCall(nil), r.call...)
}

func (r *recordingRuntime) AddInbound(ctx context.Context, ib *model.Inbound) error {
	r.record(dispatchCall{op: "add", newProto: ib.Protocol})
	return r.fakeNodeRuntime.AddInbound(ctx, ib)
}

func (r *recordingRuntime) DelInbound(ctx context.Context, ib *model.Inbound) error {
	r.record(dispatchCall{op: "del", oldProto: ib.Protocol})
	return r.fakeNodeRuntime.DelInbound(ctx, ib)
}

func (r *recordingRuntime) UpdateInbound(ctx context.Context, oldIb, newIb *model.Inbound) error {
	r.record(dispatchCall{op: "update", oldProto: oldIb.Protocol, newProto: newIb.Protocol})
	return r.fakeNodeRuntime.UpdateInbound(ctx, oldIb, newIb)
}

func awgDispatchHarness(t *testing.T, tag string, port int, settings string) *recordingRuntime {
	t.Helper()
	setupConflictDB(t)
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	rec := &recordingRuntime{}
	mgr.SetLocalRuntimeOverride(rec)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })
	seedInboundConflict(t, tag, "0.0.0.0", port, model.AWG, ``, settings)
	return rec
}

const awgDispatchSettings = `{"privateKey":"srv-priv","publicKey":"srv-pub","address":"10.200.0.1/24",` +
	`"clients":[{"id":"peer-pub-1","enable":true,"email":"a@awg","allowedIPs":["10.200.0.2/32"]}]}`

// Saving an AWG inbound used to dispatch as DelInbound + AddInbound, and
// DelInbound drops the manager's record for that id — the very record whose
// device fingerprint decides whether the kernel interface can stay up. Every
// save therefore tore the interface down and rebuilt it, dropping every peer
// session, for an edit as small as a new remark.
func TestUpdateInboundAwgDispatchesAsUpdate(t *testing.T) {
	rec := awgDispatchHarness(t, "awg-apply", 51900, awgDispatchSettings)

	update := *loadInboundByTag(t, "awg-apply")
	update.Remark = "renamed while peers stay connected"
	_, needRestart, err := (&InboundService{}).UpdateInbound(&update)
	if err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}
	if needRestart {
		t.Error("an AWG-only edit must not request an xray restart")
	}

	got := rec.calls()
	if len(got) != 1 || got[0].op != "update" {
		t.Fatalf("an AWG edit must reach the runtime as one update, got %+v", got)
	}
	if got[0].oldProto != model.AWG || got[0].newProto != model.AWG {
		t.Fatalf("both sides of the update must be AWG, got %+v", got[0])
	}
}

// The snapshot handed to the runtime carried the NEW protocol, because it was
// taken after the field was overwritten. Switching away from AWG then asked the
// Xray API to delete a tag it never had, and the kernel interface stayed up.
func TestUpdateInboundAwgToVlessCarriesTheOldProtocol(t *testing.T) {
	rec := awgDispatchHarness(t, "awg-switch", 51901, awgDispatchSettings)

	update := *loadInboundByTag(t, "awg-switch")
	update.Protocol = model.VLESS
	update.Settings = `{"clients":[{"id":"6f1d0f6e-1c2b-4a3d-8e5f-0a1b2c3d4e5f","email":"a@awg","enable":true}]}`
	update.StreamSettings = `{"network":"tcp","security":"none"}`
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	got := rec.calls()
	if len(got) == 0 {
		t.Fatal("a protocol switch must reach the runtime")
	}
	for _, c := range got {
		if c.op == "del" && c.oldProto != model.AWG {
			t.Fatalf("teardown saw protocol %q, so it never reached the AWG manager: %+v", c.oldProto, c)
		}
		if c.op == "update" && c.oldProto != model.AWG {
			t.Fatalf("update's old side must still be AWG, got %+v", c)
		}
	}
}

// AmneziaWG has carried a dedicated updateAmneziaWGInbound since it was added,
// written to reconfigure the embedded device in place — but the dispatch gate
// only ever admitted MTProto, so nothing could reach it from an inbound edit.
func TestUpdateInboundAmneziaWGDispatchesAsUpdate(t *testing.T) {
	setupConflictDB(t)
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	rec := &recordingRuntime{}
	mgr.SetLocalRuntimeOverride(rec)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	seedInboundConflict(t, "anet-apply", "0.0.0.0", 51910, model.AmneziaWG, ``,
		`{"server":{"privateKey":"priv","publicKey":"pub","subnetIp":"10.8.1.0","subnetCidr":24},`+
			`"clients":[{"email":"a@x","enable":true,"publicKey":"pub-a","allowedIPs":["10.8.1.2/32"]}]}`)

	update := *loadInboundByTag(t, "anet-apply")
	update.Remark = "renamed while peers stay connected"
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	got := rec.calls()
	if len(got) != 1 || got[0].op != "update" {
		t.Fatalf("an AmneziaWG edit must reach the runtime as one update, got %+v", got)
	}
}

// The in-place branch fixed its snapshot; the Del+Add branch beside it took the
// snapshot after the protocol field had already been overwritten, so switching
// a sidecar protocol to an Xray one asked the Xray API to delete a tag it never
// had while the sidecar kept its port.
func TestUpdateInboundSidecarToVlessCarriesTheOldProtocol(t *testing.T) {
	setupConflictDB(t)
	mgr := runtime.NewManager(runtime.LocalDeps{APIPort: func() int { return 0 }})
	rec := &recordingRuntime{}
	mgr.SetLocalRuntimeOverride(rec)
	runtime.SetManager(mgr)
	t.Cleanup(func() { runtime.SetManager(nil) })

	seedInboundConflict(t, "mieru-switch", "0.0.0.0", 51920, model.Mieru, ``,
		`{"portBindings":[{"port":51920,"protocol":"TCP"}],"clients":[]}`)

	update := *loadInboundByTag(t, "mieru-switch")
	update.Protocol = model.VLESS
	update.Settings = `{"clients":[{"id":"7f1d0f6e-1c2b-4a3d-8e5f-0a1b2c3d4e5f","email":"s@x","enable":true}]}`
	update.StreamSettings = `{"network":"tcp","security":"none"}`
	if _, _, err := (&InboundService{}).UpdateInbound(&update); err != nil {
		t.Fatalf("UpdateInbound: %v", err)
	}

	for _, c := range rec.calls() {
		if c.op == "del" && c.oldProto != model.Mieru {
			t.Fatalf("teardown saw protocol %q, so the mieru sidecar was never stopped: %+v", c.oldProto, c)
		}
	}
}
