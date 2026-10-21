// Copyright (c) 2025 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package runtime

import (
	"context"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func TestKernelWireguardPeerEditsDoNotCallXrayAPI(t *testing.T) {
	local := NewLocal(LocalDeps{APIPort: func() int { return 0 }})
	ib := &model.Inbound{Protocol: model.WireGuard, Settings: `{"kernel":true}`}
	if err := local.AddUser(context.Background(), ib, map[string]any{"email": "alice"}); err != nil {
		t.Fatalf("peer add tried Xray API: %v", err)
	}
	if err := local.RemoveUser(context.Background(), ib, "alice"); err != nil {
		t.Fatalf("peer removal tried Xray API: %v", err)
	}
}
