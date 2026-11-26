// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build linux

package tunnel

import (
	"context"
	"encoding/binary"
	"io"
	"net"
	"strconv"
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/net/proxy"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

var (
	tproxySocksMu     sync.Mutex
	tproxySocksLn     net.Listener
	tproxySocksLn6    net.Listener
	tproxySocksTarget string
)

func startTproxySocksBridge(redirPort, socksPort int) {
	if redirPort <= 0 || socksPort <= 0 {
		return
	}
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(socksPort))
	tproxySocksMu.Lock()
	defer tproxySocksMu.Unlock()
	tproxySocksTarget = addr
	if tproxySocksLn != nil {
		return
	}
	ln, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(redirPort)))
	if err != nil {
		logger.Warningf("tunnel: tproxy socks bridge listen: %v", err)
		return
	}
	tproxySocksLn = ln
	go serveTproxySocksBridge(ln)
	// IPv6 twin: the v6 REDIRECT lands on [::1], so the bridge must listen
	// there too; a v6-less host just leaves this listener down and the v6
	// REDIRECT (if any was added) fails closed.
	ln6, err := (&net.ListenConfig{}).Listen(context.Background(), "tcp", net.JoinHostPort("::1", strconv.Itoa(redirPort)))
	if err != nil {
		logger.Warningf("tunnel: tproxy socks bridge listen v6: %v", err)
		return
	}
	tproxySocksLn6 = ln6
	go serveTproxySocksBridge(ln6)
}

func serveTproxySocksBridge(ln net.Listener) {
	for {
		c, err := ln.Accept()
		if err != nil {
			return
		}
		go handleTproxySocksConn(c)
	}
}

func handleTproxySocksConn(c net.Conn) {
	defer c.Close()
	dst, err := originalTCPDest(c)
	if err != nil || dst == "" {
		return
	}
	tproxySocksMu.Lock()
	socks := tproxySocksTarget
	tproxySocksMu.Unlock()
	if socks == "" {
		return
	}
	dialer, err := proxy.SOCKS5("tcp", socks, nil, proxy.Direct)
	if err != nil {
		return
	}
	up, err := dialer.Dial("tcp", dst)
	if err != nil {
		return
	}
	defer up.Close()
	go func() { _, _ = io.Copy(up, c) }()
	_, _ = io.Copy(c, up)
}

func originalTCPDest(c net.Conn) (string, error) {
	tc, ok := c.(*net.TCPConn)
	if !ok {
		return "", syscall.EINVAL
	}
	// The peer's source family is the socket family of the redirected
	// connection, so it tells which SO_ORIGINAL_DST shape to read (and a
	// dial to a v6 target is always made from an AF_INET6 socket).
	if ra, ok := c.RemoteAddr().(*net.TCPAddr); ok && ra.IP.To4() == nil {
		return originalTCPDest6(tc)
	}
	rc, err := tc.SyscallConn()
	if err != nil {
		return "", err
	}
	var ip syscall.RawSockaddrInet4
	var errno syscall.Errno
	err = rc.Control(func(fd uintptr) {
		var n uint32 = syscall.SizeofSockaddrInet4
		_, _, errno = syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			syscall.IPPROTO_IP,
			80,
			uintptr(unsafe.Pointer(&ip)),
			uintptr(unsafe.Pointer(&n)),
			0,
		)
	})
	if err != nil {
		return "", err
	}
	if errno != 0 {
		return "", errno
	}
	port := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&ip.Port))[:])
	host := net.IPv4(ip.Addr[0], ip.Addr[1], ip.Addr[2], ip.Addr[3]).String()
	return net.JoinHostPort(host, strconv.Itoa(int(port))), nil
}

func originalTCPDest6(tc *net.TCPConn) (string, error) {
	rc, err := tc.SyscallConn()
	if err != nil {
		return "", err
	}
	var ip syscall.RawSockaddrInet6
	var errno syscall.Errno
	err = rc.Control(func(fd uintptr) {
		var n uint32 = syscall.SizeofSockaddrInet6
		_, _, errno = syscall.Syscall6(
			syscall.SYS_GETSOCKOPT,
			fd,
			syscall.IPPROTO_IP,
			80,
			uintptr(unsafe.Pointer(&ip)),
			uintptr(unsafe.Pointer(&n)),
			0,
		)
	})
	if err != nil {
		return "", err
	}
	if errno != 0 {
		return "", errno
	}
	port := binary.BigEndian.Uint16((*[2]byte)(unsafe.Pointer(&ip.Port))[:])
	return net.JoinHostPort(net.IP(ip.Addr[:]).String(), strconv.Itoa(int(port))), nil
}
