// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

//go:build windows

package tunnel

import (
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

var (
	killOnExitJobOnce sync.Once
	killOnExitJob     windows.Handle
	killOnExitJobErr  error
)

func ensureKillOnExitJob() (windows.Handle, error) {
	killOnExitJobOnce.Do(func() {
		h, err := windows.CreateJobObject(nil, nil)
		if err != nil {
			killOnExitJobErr = err
			return
		}
		info := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{
			BasicLimitInformation: windows.JOBOBJECT_BASIC_LIMIT_INFORMATION{
				LimitFlags: windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE,
			},
		}
		_, err = windows.SetInformationJobObject(
			h,
			windows.JobObjectExtendedLimitInformation,
			uintptr(unsafe.Pointer(&info)),
			uint32(unsafe.Sizeof(info)),
		)
		if err != nil {
			_ = windows.CloseHandle(h)
			killOnExitJobErr = err
			return
		}
		killOnExitJob = h
	})
	return killOnExitJob, killOnExitJobErr
}

func attachChildLifetime(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	job, err := ensureKillOnExitJob()
	if err != nil {
		logger.Warningf("tunnel: kill-on-exit job unavailable: %v", err)
		return
	}
	h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
	if err != nil {
		logger.Warningf("tunnel: OpenProcess for job attach failed: %v", err)
		return
	}
	defer func() { _ = windows.CloseHandle(h) }()
	if err := windows.AssignProcessToJobObject(job, h); err != nil {
		logger.Warningf("tunnel: AssignProcessToJobObject failed: %v", err)
	}
}
