// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package model

import (
	"reflect"
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestHostJSONColumnsReadLeniently(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&Host{}); err != nil {
		t.Fatal(err)
	}
	// A normal write still stores JSON.
	good := Host{InboundId: 1, Remark: "good", Alpn: []string{"h2"}, Tags: []string{"a"}}
	if err := db.Create(&good).Error; err != nil {
		t.Fatal(err)
	}
	var stored string
	db.Raw("SELECT alpn FROM hosts WHERE id = ?", good.Id).Scan(&stored)
	if stored != `["h2"]` {
		t.Errorf("write format changed: %s", stored)
	}
	// A hand-edited row with a non-JSON list, and one with garbage.
	db.Exec(`INSERT INTO hosts (inbound_id, remark, alpn, tags) VALUES (2, 'bad', 'h2,http/1.1', '{oops')`)

	var hosts []Host
	if err := db.Order("id").Find(&hosts).Error; err != nil {
		t.Fatalf("one malformed row must not fail the query: %v", err)
	}
	if len(hosts) != 2 {
		t.Fatalf("got %d rows, want 2", len(hosts))
	}
	if !reflect.DeepEqual(hosts[0].Alpn, []string{"h2"}) {
		t.Errorf("good row alpn = %v", hosts[0].Alpn)
	}
	if !reflect.DeepEqual(hosts[1].Alpn, []string{"h2", "http/1.1"}) {
		t.Errorf("comma list not recovered: %v", hosts[1].Alpn)
	}
	if !reflect.DeepEqual(hosts[1].Tags, []string{"{oops"}) {
		t.Errorf("garbage tags = %v", hosts[1].Tags)
	}
}
