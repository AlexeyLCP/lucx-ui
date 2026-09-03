// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package awg

func renameAwgInterfaceSeq(set func(args ...string) error, oldName, newName string) error {
	if oldName == "" || oldName == newName {
		return nil
	}
	downErr := set(oldName, "down")
	if err := set(oldName, "name", newName); err != nil {
		if downErr == nil {
			_ = set(oldName, "up")
		}
		return err
	}
	return set(newName, "up")
}
