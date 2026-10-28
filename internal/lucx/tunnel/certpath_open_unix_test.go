// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build unix

package tunnel

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// A non-regular path is refused by Stat before any open, as in lucx.318: opening a
// device has side effects. A FIFO writer blocked in open wakes only if we open it.
func TestNonRegularPathRefusedBeforeOpen(t *testing.T) {
	fifo := filepath.Join(t.TempDir(), "cert.pem")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	writerOpened := make(chan struct{})
	go func() {
		if f, err := os.OpenFile(fifo, os.O_WRONLY, 0); err == nil {
			close(writerOpened)
			f.Close()
		}
	}()
	time.Sleep(200 * time.Millisecond)
	if _, err := readRegularFile(fifo, maxCertFileBytes); err == nil || err.Error() != "not a regular file" {
		t.Fatalf("readRegularFile(fifo) = %v, want \"not a regular file\"", err)
	}
	select {
	case <-writerOpened:
		t.Fatal("the FIFO was opened before its type was checked")
	case <-time.After(time.Second):
	}
	if r, err := os.OpenFile(fifo, os.O_RDONLY|syscall.O_NONBLOCK, 0); err == nil {
		<-writerOpened
		r.Close()
	}
}
