// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package plugin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/mhsanaei/3x-ui/v3/internal/eventbus"
)

/* Stage 3b: plugins that declare `events` receive panel eventbus events. The
contract borrows the community plugin system's vocabulary and delivery rules
and reuses the event bus primitives: one bus subscriber per plugin (its own
64-event queue and serial worker give the per-plugin delivery order for
free). Delivery is an HTTP POST to the plugin's unix socket at
/_lucx/events — best effort, 5 seconds, no retry; the panel counts
delivered and failed payloads only (bus-side queue drops are visible in the
panel log). */

const socketEventsEndpoint = "/_lucx/events"

/*
	eventVocabulary mirrors the publisher set of internal/eventbus: the

community system's client.* and inbound.* names have no publishers in this
panel yet, so declaring them would only produce a silent stream of
nothing.
*/
var eventVocabulary = map[string]bool{
	string(eventbus.EventOutboundDown): true,
	string(eventbus.EventOutboundUp):   true,
	string(eventbus.EventXrayCrash):    true,
	string(eventbus.EventNodeDown):     true,
	string(eventbus.EventNodeUp):       true,
	string(eventbus.EventCPUHigh):      true,
	string(eventbus.EventMemoryHigh):   true,
	string(eventbus.EventLoginAttempt): true,
}

type EventsStatus struct {
	Delivered int64  `json:"delivered"`
	Failed    int64  `json:"failed"`
	Socket    string `json:"socket,omitempty"`
}

type eventEnvelope struct {
	ID     string          `json:"id"`
	Type   string          `json:"type"`
	Source string          `json:"source,omitempty"`
	Time   string          `json:"time"`
	Data   json.RawMessage `json:"data,omitempty"`
}

var (
	busLock     sync.Mutex
	busInstance *eventbus.Bus
	subLock     sync.Mutex
	subscribed  = map[string]bool{}
	runLock     sync.Mutex
	runs        = map[string]*runState{}
)

type runState struct {
	delivered atomic.Int64
	failed    atomic.Int64
	socket    string
}

func runStateOf(id string) *runState {
	runLock.Lock()
	defer runLock.Unlock()
	return runs[id]
}

// SetEventBus is the wiring point called by the panel once at startup.
func SetEventBus(b *eventbus.Bus) {
	busLock.Lock()
	busInstance = b
	busLock.Unlock()
}

func busRef() *eventbus.Bus {
	busLock.Lock()
	defer busLock.Unlock()
	return busInstance
}

// validateEvents checks the manifest events list against the vocabulary.
func validateEvents(names []string) error {
	if len(names) > 16 {
		return errors.New("plugin manifest allows at most 16 event names")
	}
	seen := make(map[string]bool, len(names))
	for _, n := range names {
		if !eventVocabulary[n] {
			return fmt.Errorf("unknown plugin event %q in apiVersion 1", n)
		}
		if seen[n] {
			return fmt.Errorf("plugin event %q is listed twice", n)
		}
		seen[n] = true
	}
	return nil
}

// socketPath returns the absolute unix socket the plugin should bind. The
// unit runner gets a systemd RuntimeDirectory; the child runner a socket
// inside its own 0700 plugin directory. Both are owned by the panel's user.
func socketPath(m Manifest, runner string) string {
	if runner == runnerUnit {
		return filepath.Join("/run", "x-ui-plug-"+m.ID, m.ID+".sock")
	}
	plugDir, err := filepath.Abs(dir(m.ID))
	if err != nil {
		plugDir = dir(m.ID)
	}
	return filepath.Join(plugDir, "plugin.sock")
}

// registerSocket records where a running plugin's socket lives; the
// delivery goroutines read it. Register before the process starts so the
// subscriber never sees a missing entry.
func registerSocket(id, path string) *runState {
	runLock.Lock()
	defer runLock.Unlock()
	rs := runs[id]
	if rs == nil {
		rs = &runState{}
		runs[id] = rs
	}
	rs.socket = path
	return rs
}

func forgetSocket(id string) {
	runLock.Lock()
	defer runLock.Unlock()
	delete(runs, id)
}

func eventsStatus(id string) *EventsStatus {
	rs := runStateOf(id)
	if rs == nil {
		return nil
	}
	return &EventsStatus{Delivered: rs.delivered.Load(), Failed: rs.failed.Load(), Socket: rs.socket}
}

/*
	ensureEventSubscriber registers the bus subscription for a running plugin.

Subscribed ids are tracked apart from managed processes so a plugin unit
that outlived a panel restart is re-subscribed by the next status poll;
Subscribe with a known id replaces its subscriber, so repeats are harmless.
*/
func ensureEventSubscriber(m Manifest, runner string) {
	if len(m.Events) == 0 {
		return
	}
	subLock.Lock()
	known := subscribed[m.ID]
	subscribed[m.ID] = true
	subLock.Unlock()
	if known {
		return
	}
	if b := busRef(); b != nil {
		b.Subscribe("plugins:"+m.ID, func(e eventbus.Event) {
			deliverEvent(m, runner, e)
		})
	}
}

func removeEventSubscriber(id string) {
	subLock.Lock()
	was := subscribed[id]
	delete(subscribed, id)
	subLock.Unlock()
	if was {
		if b := busRef(); b != nil {
			b.Unsubscribe("plugins:" + id)
		}
	}
	forgetSocket(id)
}

/*
	deliverEvent posts one event to the plugin socket. The bus worker calls it

serially per plugin, so counters need only atomics. A subscriber created
after a panel restart (unit still running) computes the socket path on
first delivery.
*/
func deliverEvent(m Manifest, runner string, e eventbus.Event) {
	if !containsString(m.Events, string(e.Type)) {
		return
	}
	rs := runStateOf(m.ID)
	if rs == nil || rs.socket == "" {
		rs = registerSocket(m.ID, socketPath(m, runner))
	}
	raw, err := json.Marshal(e.Data)
	if err != nil {
		rs.failed.Add(1)
		_, _ = appendPluginLog(m.ID, "event "+string(e.Type)+" payload is not marshalable")
		return
	}
	payload, err := json.Marshal(eventEnvelope{
		ID:     eventID(e),
		Type:   string(e.Type),
		Source: e.Source,
		Time:   e.Timestamp.UTC().Format(time.RFC3339Nano),
		Data:   raw,
	})
	if err != nil {
		rs.failed.Add(1)
		_, _ = appendPluginLog(m.ID, "event "+string(e.Type)+" envelope is not marshalable")
		return
	}
	if err := postToSocket(rs.socket, payload); err != nil {
		rs.failed.Add(1)
		_, _ = appendPluginLog(m.ID, "event "+string(e.Type)+" delivery failed: "+err.Error())
		return
	}
	rs.delivered.Add(1)
}

func postToSocket(path string, payload []byte) error {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", path)
		},
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: tr}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost,
		"http://unix"+socketEventsEndpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
	tr.CloseIdleConnections()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("plugin answered %q", resp.Status)
	}
	return nil
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

func eventID(e eventbus.Event) string {
	return e.Timestamp.UTC().Format(time.RFC3339Nano) + ":" + strings.ReplaceAll(e.Source, ":", "-")
}
