// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

// Package caddylucx bridges Caddy's layer4 app into the HTTP app without a
// socket hop. The l4chan network provides named in-memory listeners that HTTP
// servers bind via `bind l4chan/<name>`; the l4http layer4 handler pushes a
// matched connection into such a listener and blocks until the HTTP side
// closes it — the TLS ClientHello the SNI matcher peeked at is replayed to
// the HTTP server's TLS terminator, and RemoteAddr stays the real client.
package caddylucx

import (
	"context"
	"fmt"
	"net"
	"sync"
	"sync/atomic"

	"github.com/caddyserver/caddy/v2"
	"github.com/caddyserver/caddy/v2/caddyconfig/caddyfile"
	"github.com/mholt/caddy-l4/layer4"
)

func init() {
	caddy.RegisterNetwork("l4chan", newChanListener)
	caddy.RegisterModule(&HTTPBridge{})
}

// chanAddr is the net.Addr reported by channel listeners; the address itself
// is only a routing name — connections never touch a socket.
type chanAddr string

func (a chanAddr) Network() string { return "l4chan" }
func (a chanAddr) String() string  { return string(a) }

var (
	chanMu   sync.RWMutex
	chanLive = map[string]*chanListener{}
)

// chanListener is a net.Listener fed by l4http handlers. Reloads replace the
// registry entry: in-flight connections keep draining on the old listener
// while new ones go to the fresh server's Accept loop.
type chanListener struct {
	name  string
	conns chan net.Conn
	done  chan struct{}
	once  sync.Once
}

func newChanListener(_ context.Context, _, host, _ string, _ uint, _ net.ListenConfig) (any, error) {
	if host == "" {
		return nil, fmt.Errorf("l4chan: listener name required")
	}
	l := &chanListener{name: host, conns: make(chan net.Conn, 128), done: make(chan struct{})}
	chanMu.Lock()
	chanLive[host] = l
	chanMu.Unlock()
	return l, nil
}

func (l *chanListener) Accept() (net.Conn, error) {
	select {
	case c := <-l.conns:
		if bc, ok := c.(*bridgeConn); ok {
			bc.accepted.Store(true)
		}
		return c, nil
	case <-l.done:
		return nil, net.ErrClosed
	}
}

func (l *chanListener) Close() error {
	l.once.Do(func() {
		close(l.done)
		chanMu.Lock()
		if chanLive[l.name] == l {
			delete(chanLive, l.name)
		}
		chanMu.Unlock()
	})
	return nil
}

func (l *chanListener) Addr() net.Addr { return chanAddr(l.name) }

// bridgeConn hands a layer4 connection to the HTTP accept loop. Its done
// channel fires when the HTTP side closes the connection so the l4 handler
// (which must not return early — the app closes the conn afterwards) can
// release it.
type bridgeConn struct {
	net.Conn
	done     chan struct{}
	once     sync.Once
	accepted atomic.Bool
}

func (c *bridgeConn) Close() error {
	err := c.Conn.Close()
	c.once.Do(func() { close(c.done) })
	return err
}

// HTTPBridge is the `l4http` layer4 handler: it injects the connection into
// the named l4chan listener that an http server block is bound to.
type HTTPBridge struct {
	Name string `json:"name,omitempty"`
}

func (*HTTPBridge) CaddyModule() caddy.ModuleInfo {
	return caddy.ModuleInfo{
		ID:  "layer4.handlers.l4http",
		New: func() caddy.Module { return new(HTTPBridge) },
	}
}

// Handle pushes the connection into the channel listener, then blocks until
// the HTTP server closes it — layer4 closes the conn once the route chain
// returns, so returning early would kill the bridged session.
func (h *HTTPBridge) Handle(cx *layer4.Connection, _ layer4.Handler) error {
	chanMu.RLock()
	l := chanLive[h.Name]
	chanMu.RUnlock()
	if l == nil {
		return fmt.Errorf("l4http: no http listener %q", h.Name)
	}
	bc := &bridgeConn{Conn: cx, done: make(chan struct{})}
	select {
	case l.conns <- bc:
	case <-l.done:
		return fmt.Errorf("l4http: listener %q closed", h.Name)
	}
	select {
	case <-bc.done:
	case <-l.done:
		// The listener died: a conn still queued is ours to release, but an
		// already-accepted one belongs to the draining HTTP server — closing
		// it here would kill in-flight requests on graceful reload.
		if bc.accepted.Load() {
			<-bc.done
		} else {
			_ = bc.Close()
		}
	}
	return nil
}

// UnmarshalCaddyfile parses `l4http <name>`.
func (h *HTTPBridge) UnmarshalCaddyfile(d *caddyfile.Dispenser) error {
	d.Next()
	if !d.NextArg() {
		return d.ArgErr()
	}
	h.Name = d.Val()
	return nil
}

var (
	_ layer4.NextHandler    = (*HTTPBridge)(nil)
	_ caddyfile.Unmarshaler = (*HTTPBridge)(nil)
	_ net.Addr              = chanAddr("")
)
