// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

func eventsFixture(t *testing.T, id string, handler http.HandlerFunc) string {
	td := t.TempDir()
	old := binFolder
	binFolder = func() string { return td }
	t.Cleanup(func() { binFolder = old })
	plugDir := dir(id)
	if err := os.MkdirAll(plugDir, 0o700); err != nil {
		t.Fatal(err)
	}
	sock := filepath.Join(td, id+".sock")
	ln, err := net.Listen("unix", sock)
	if err != nil {
		t.Fatal(err)
	}
	srv := &http.Server{Handler: handler}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(func() {
		_ = srv.Close()
		_ = ln.Close()
		removeEventSubscriber(id)
		forgetSocket(id)
	})
	return sock
}

func TestValidateEvents(t *testing.T) {
	if err := validateEvents([]string{"cpu.high", "xray.crash"}); err != nil {
		t.Fatalf("valid vocabulary rejected: %v", err)
	}
	if err := validateEvents([]string{"client.created"}); err == nil {
		t.Fatal("an event without a publisher was accepted")
	}
	if err := validateEvents([]string{"cpu.high", "cpu.high"}); err == nil {
		t.Fatal("a duplicate event was accepted")
	}
	long := make([]string, 17)
	for i := range long {
		long[i] = "cpu.high"
	}
	if err := validateEvents(long); err == nil {
		t.Fatal("an oversized events list was accepted")
	}
	for _, name := range []string{string(eventbus.EventXrayCrash), string(eventbus.EventNodeDown), string(eventbus.EventLoginAttempt), string(eventbus.EventMemoryHigh)} {
		if !eventVocabulary[name] {
			t.Fatalf("event %q missing from the vocabulary", name)
		}
	}
}

func TestSocketPathPerRunner(t *testing.T) {
	td := t.TempDir()
	old := binFolder
	binFolder = func() string { return td }
	t.Cleanup(func() { binFolder = old })
	m := Manifest{ID: "ev-path"}
	if got := socketPath(m, runnerUnit); got != filepath.Join("/run", "x-ui-plug-"+m.ID, m.ID+".sock") {
		t.Fatalf("unit socket path wrong: %v", got)
	}
	child := socketPath(m, runnerProcess)
	if !filepath.IsAbs(child) || filepath.Base(child) != "plugin.sock" {
		t.Fatalf("child socket path wrong: %v", child)
	}
}

func TestDeliverEventPostsEnvelope(t *testing.T) {
	var mu sync.Mutex
	var gotBody eventEnvelope
	gotPath := make(chan string, 1)
	sock := eventsFixture(t, "ev-test-a", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		var e eventEnvelope
		_ = json.Unmarshal(b, &e)
		mu.Lock()
		gotBody = e
		mu.Unlock()
		gotPath <- r.URL.Path
		w.WriteHeader(http.StatusOK)
	})
	registerSocket("ev-test-a", sock)
	m := Manifest{ID: "ev-test-a", Events: []string{"cpu.high"}}
	deliverEvent(m, runnerProcess, eventbus.Event{Type: eventbus.EventCPUHigh, Source: "cpu0", Data: &eventbus.SystemMetricData{Percent: 12.5}})
	path := <-gotPath
	if path != socketEventsEndpoint {
		t.Fatalf("events posted to %q", path)
	}
	mu.Lock()
	defer mu.Unlock()
	if gotBody.Type != "cpu.high" || gotBody.Source != "cpu0" {
		t.Fatalf("envelope payload wrong: %+v", gotBody)
	}
	if gotBody.Time == "" || gotBody.ID == "" {
		t.Fatal("envelope lacks id/time")
	}
	if s := eventsStatus("ev-test-a"); s == nil || s.Delivered != 1 || s.Failed != 0 {
		t.Fatalf("delivered counter wrong: %+v", s)
	}
}

func TestDeliverEventCountsFailure(t *testing.T) {
	sock := eventsFixture(t, "ev-test-b", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	registerSocket("ev-test-b", sock)
	m := Manifest{ID: "ev-test-b", Events: []string{"cpu.high"}}
	deliverEvent(m, runnerProcess, eventbus.Event{Type: eventbus.EventXrayCrash, Source: "s", Data: "boom"})
	// xray.crash is not in the enabled list: nothing counted.
	if s := eventsStatus("ev-test-b"); s == nil || s.Delivered != 0 || s.Failed != 0 {
		t.Fatalf("filtered event changed counters: %+v", s)
	}
	deliverEvent(m, runnerProcess, eventbus.Event{Type: eventbus.EventCPUHigh, Data: "x"})
	if s := eventsStatus("ev-test-b"); s == nil || s.Delivered != 0 || s.Failed != 1 {
		t.Fatalf("failed counter wrong: %+v", s)
	}
}

func TestDeliverEventNoListenerFails(t *testing.T) {
	sock := eventsFixture(t, "ev-test-c", func(http.ResponseWriter, *http.Request) {})
	_ = os.Remove(sock)
	registerSocket("ev-test-c", sock)
	m := Manifest{ID: "ev-test-c", Events: []string{"cpu.high"}}
	deliverEvent(m, runnerProcess, eventbus.Event{Type: eventbus.EventCPUHigh})
	if s := eventsStatus("ev-test-c"); s == nil || s.Failed != 1 {
		t.Fatalf("missing socket must count as failure: %+v", s)
	}
}

func TestEnsureSubscriberDeliversAndStops(t *testing.T) {
	var mu sync.Mutex
	received := 0
	sock := eventsFixture(t, "ev-test-d", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		mu.Lock()
		received++
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	})
	registerSocket("ev-test-d", sock)
	bus := eventbus.New(0)
	SetEventBus(bus)
	t.Cleanup(func() { SetEventBus(nil) })
	m := Manifest{ID: "ev-test-d", Events: []string{"cpu.high"}}
	ensureEventSubscriber(m, runnerProcess)
	ensureEventSubscriber(m, runnerProcess)
	bus.Publish(eventbus.Event{Type: eventbus.EventCPUHigh, Data: "x"})
	deadline := time.Now().Add(3 * time.Second)
	for {
		mu.Lock()
		n := received
		mu.Unlock()
		if n > 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("event never delivered through the bus subscriber")
		}
		time.Sleep(50 * time.Millisecond)
	}
	mu.Lock()
	seen := received
	mu.Unlock()
	removeEventSubscriber("ev-test-d")
	bus.Publish(eventbus.Event{Type: eventbus.EventCPUHigh, Data: "x"})
	time.Sleep(300 * time.Millisecond)
	mu.Lock()
	after := received
	mu.Unlock()
	if after != seen {
		t.Fatalf("subscriber was not removed: %d -> %d", seen, after)
	}
}
