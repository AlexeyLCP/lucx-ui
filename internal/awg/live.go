// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

// LivePeer is one kernel-interface peer as shown by `awg show dump`.
type LivePeer struct {
	InboundId     int
	Ifname        string
	Tag           string
	Email         string
	PublicKey     string
	Endpoint      string
	AllowedIPs    string
	Rx            int64
	Tx            int64
	LastHandshake int64
}

// HasRunning reports whether any LucX-managed kernel AWG interface is up.
func (m *Manager) HasRunning() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.procs) > 0
}

// LivePeers scrapes every running kernel interface and maps pubkeys to emails
// from the last Ensure/Reconcile peer list.
func (m *Manager) LivePeers() []LivePeer {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []LivePeer
	for id, s := range m.procs {
		if s == nil || s.ifname == "" {
			continue
		}
		stats, ok := scrapePeers(s.ifname)
		if !ok {
			continue
		}
		emailByPub := map[string]string{}
		allowedByPub := map[string]string{}
		for _, p := range s.peers {
			emailByPub[p.PublicKey] = p.Email
			allowedByPub[p.PublicKey] = p.AllowedIPs
		}
		for _, st := range stats {
			allowed := st.AllowedIPs
			if allowed == "" {
				allowed = allowedByPub[st.PublicKey]
			}
			out = append(out, LivePeer{
				InboundId:     id,
				Ifname:        s.ifname,
				Tag:           s.tag,
				Email:         emailByPub[st.PublicKey],
				PublicKey:     st.PublicKey,
				Endpoint:      st.Endpoint,
				AllowedIPs:    allowed,
				Rx:            st.Rx,
				Tx:            st.Tx,
				LastHandshake: st.LastHandshake,
			})
		}
	}
	return out
}
