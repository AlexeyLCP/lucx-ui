# Kernel AWG TPROXY

`routeThroughXray` stays the gate. False is direct. True with a missing or `tun`
`xrayRoutingMode` is the existing TUN bridge. True with `tproxy` is a loopback
dokodemo-door on `tproxyPort`. No migration rewrites client keys or addresses.

Local kernel AWG only. IPv4 TCP/UDP. Server subnet /8 through /30. Peer prefixes
must sit inside that subnet. No remote nodes, no IPv6, no userspace fallback.
Internet ICMP and other forwarded protocols are dropped. P2P still follows the
existing option, including ICMP between peers.

Each inbound N owns chain `LXATN`, mark `0x40000000+N`, policy rule priority
10000 on that interface, and table `40000+N` with route protocol 242. A foreign
route in that table or a foreign rule in the chain fails the apply instead of
being overwritten. Firewall rules use comment `lucx-awg-tproxy`. Host OUTPUT
and the default route are not changed. `ip_forward` is not reset on cleanup.

The hidden bridge keeps the inbound tag, sniffing with `routeOnly`, and the
selected outbound or balancer. It listens only on 127.0.0.1.

Linux netns check:

```sh
go test -c -o /tmp/lucx-awg-test ./internal/awg
sudo env LUCX_TPROXY_NETNS_TEST=1 /tmp/lucx-awg-test -test.run '^TestTproxyNetns$' -test.v
```
