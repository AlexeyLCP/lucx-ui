package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
)

func naiveGuardFixture(id int, remark, domain string, behindCover bool) *model.Inbound {
	m := map[string]any{"domain": domain, "behindCover": behindCover}
	if behindCover {
		m["hideOn443"] = true
	}
	raw, _ := json.Marshal(m)
	return &model.Inbound{Id: id, Protocol: model.Naive, Remark: remark, Enable: true, Settings: string(raw)}
}

func TestGuardOneNaivePerCover(t *testing.T) {
	host := "site.example.com"
	byID := map[int]*model.Inbound{
		4: naiveGuardFixture(4, "NaiveFI", host, true),
		7: naiveGuardFixture(7, "NaiveNLv", "", false),
	}

	if err := guardOneNaivePerCover(byID, host, nil); err != nil {
		t.Fatalf("a single hidden naive stays allowed: %v", err)
	}

	err := guardOneNaivePerCover(byID, host, []int{7})
	if err == nil {
		t.Fatal("a second naive behind the same cover must be rejected")
	}
	for _, want := range []string{"NaiveFI", "NaiveNLv", "only one Naive"} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("error must name both naives and the constraint, got %q", err.Error())
		}
	}

	plain := naiveGuardFixture(9, "NaivePlain", "", false)
	if err := guardOneNaivePerCover(map[int]*model.Inbound{9: plain}, host, nil); err != nil {
		t.Fatalf("a plain unhidden naive must not trip the guard: %v", err)
	}
}
