// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package caddylucx

import (
	"errors"
	"net"
	"testing"
	"time"

	"github.com/mholt/caddy-l4/layer4"
)

func mustListener(t *testing.T, name string) *chanListener {
	t.Helper()
	any, err := newChanListener(t.Context(), "l4chan", name, "", 0, net.ListenConfig{})
	if err != nil {
		t.Fatalf("newChanListener: %v", err)
	}
	l, ok := any.(*chanListener)
	if !ok {
		t.Fatalf("listener type %T", any)
	}
	t.Cleanup(func() { _ = l.Close() })
	return l
}

func TestChanListenerAcceptClose(t *testing.T) {
	l := mustListener(t, "t-accept")
	srv, cli := net.Pipe()
	defer cli.Close()
	l.conns <- srv
	got, err := l.Accept()
	if err != nil || got != srv {
		t.Fatalf("accept: %v %v", got, err)
	}
	if got.RemoteAddr() != srv.RemoteAddr() {
		t.Fatalf("remote addr lost")
	}
	if err := l.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := l.Accept(); !errors.Is(err, net.ErrClosed) {
		t.Fatalf("closed accept: %v", err)
	}
}

func TestChanListenerReplacedOnRelisten(t *testing.T) {
	old := mustListener(t, "t-replace")
	fresh := mustListener(t, "t-replace")
	chanMu.RLock()
	got := chanLive["t-replace"]
	chanMu.RUnlock()
	if got != fresh {
		t.Fatalf("registry kept stale listener")
	}
	srv, _ := net.Pipe()
	defer srv.Close()
	old.conns <- srv // queued on the dead listener drains via Accept
	if c, err := old.Accept(); err != nil || c != srv {
		t.Fatalf("old accept: %v %v", c, err)
	}
	_ = old.Close()
}

func TestHandleMissingListener(t *testing.T) {
	srv, cli := net.Pipe()
	defer srv.Close()
	defer cli.Close()
	h := &HTTPBridge{Name: "t-missing"}
	if err := h.Handle(&layer4.Connection{Conn: srv}, nil); err == nil {
		t.Fatalf("expected missing-listener error")
	}
}

func TestHandleServesUntilClose(t *testing.T) {
	l := mustListener(t, "t-serve")
	srv, cli := net.Pipe()
	defer cli.Close()
	h := &HTTPBridge{Name: "t-serve"}
	done := make(chan error, 1)
	go func() { done <- h.Handle(&layer4.Connection{Conn: srv}, nil) }()
	c, err := l.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	go func() {
		_, _ = c.Write([]byte("ping"))
	}()
	buf := make([]byte, 4)
	if _, err := cli.Read(buf); err != nil || string(buf) != "ping" {
		t.Fatalf("pipe through: %v %q", err, buf)
	}
	select {
	case <-done:
		t.Fatalf("handler returned before HTTP side closed the conn")
	case <-time.After(50 * time.Millisecond):
	}
	_ = c.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handle: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("handler stuck after conn close")
	}
}

func TestHandleQueuedConnReleasedOnListenerClose(t *testing.T) {
	l := mustListener(t, "t-queued")
	srv, cli := net.Pipe()
	defer cli.Close()
	h := &HTTPBridge{Name: "t-queued"}
	done := make(chan error, 1)
	go func() { done <- h.Handle(&layer4.Connection{Conn: srv}, nil) }()
	time.Sleep(30 * time.Millisecond) // let Handle enqueue first
	_ = l.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handle: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("handler stuck on closed listener")
	}
	if _, err := srv.Write([]byte("x")); err == nil {
		t.Fatalf("queued conn was not released")
	}
}

func TestHandleAcceptedConnSurvivesListenerClose(t *testing.T) {
	l := mustListener(t, "t-accepted")
	srv, cli := net.Pipe()
	defer cli.Close()
	defer srv.Close()
	h := &HTTPBridge{Name: "t-accepted"}
	done := make(chan error, 1)
	go func() { done <- h.Handle(&layer4.Connection{Conn: srv}, nil) }()
	c, err := l.Accept()
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	_ = l.Close() // reload kills the listener; the in-flight conn must stay up
	select {
	case <-done:
		t.Fatalf("handler dropped an accepted conn on listener close")
	case <-time.After(50 * time.Millisecond):
	}
	_ = c.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("handle: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("handler stuck after conn close")
	}
}
