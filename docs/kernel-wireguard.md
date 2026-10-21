# Kernel WireGuard

WireGuard inbounds can opt into the Linux kernel backend with `settings.kernel: true`. Existing inbounds continue using Xray until explicitly changed. This backend is ordinary, unobfuscated WireGuard.

Requirements: Linux WireGuard support, `ip`, `wg`, `ss`, `nft`, `sysctl`, and the network administration privileges already required by the kernel AWG manager. The panel stores private configuration files in `XUI_DB_FOLDER/wireguard` with restricted permissions.

Create a WireGuard inbound, select Linux kernel, set the server IPv4 CIDR and an unused loopback TPROXY port, then add clients normally. The first version accepts IPv4 client /32 addresses in the server subnet. It sends TCP and UDP through the inbound's Xray TPROXY bridge and existing routing rules. It does not change the host default route or DNS. IPv6 is blocked on the managed interface. Unsupported traffic is dropped rather than forwarded around Xray.

Peer changes use `wg syncconf` and preserve other peer sessions. Changing server/device settings may recreate the interface. Disabling or deleting an inbound removes only its owned interface, routing rule, route, nftables table and configuration. Reconciliation retries failed configuration and cleanup operations.

The import preview accepts a standard server configuration, rejects shell hooks and AWG parameters, and returns a disabled draft for review. Save explicitly to create the inbound. Imported public keys do not recover client private keys; exports remain unavailable for those peers until the operator supplies their original private key. Existing external interfaces are not adopted automatically.

## Validation

Run the ordinary Go tests and frontend checks. On a disposable privileged Linux runner, install wireguard-tools, nftables and iproute2, then run:

```sh
sudo env PATH="$PATH" LUCX_WG_NETNS_TEST=1 go test -v -count=1 ./internal/lucx/wireguard -run '^TestKernelWireguardNetNS$'
```

The namespace test verifies real kernel handshakes, TCP/UDP TPROXY, compatibility with a connection-mark restore chain, reconciliation, closed forwarding when the listener is absent, and preservation of an unrelated WireGuard interface. `.github/workflows/kernel-wireguard.yml` runs this test and builds a Linux validation artifact.

For deployment, back up the database and binary, retain a tested rollback, and first validate a separate synthetic inbound. Verify DNS, HTTPS, routing, peer revocation, traffic accounting and existing tunnels before transferring externally managed interfaces.
