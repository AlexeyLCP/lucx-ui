package service

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"gorm.io/gorm"

	"github.com/mhsanaei/3x-ui/v3/internal/database"
	"github.com/mhsanaei/3x-ui/v3/internal/database/model"
	"github.com/mhsanaei/3x-ui/v3/internal/xray"
)

// TestMigrationRequirements_BackfillsClientTrafficsWithMultiDomainInbound guards the
// PostgreSQL fix where the externalProxy detection query (executed via .Scan) errored on
// json_extract and rolled back the whole transaction — including the client_traffics
// backfill at inbound.go:3093-3106, leaving clients with no traffic rows. A MultiDomain
// inbound is present so that query returns rows and the function runs to completion; both
// the backfill and the MultiDomain→ExternalProxy migration must then commit.
func TestMigrationRequirements_BackfillsClientTrafficsWithMultiDomainInbound(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()

	const backfillEmail = "needsbackfill@example.com"
	const uid = "ce8d33df-3a64-4f10-8f9b-91c3a8e0c010"

	// Inbound A: a client present only in settings.clients, with no client_traffics row.
	clientInbound := &model.Inbound{
		UserId:         1,
		Tag:            "a-tag",
		Enable:         true,
		Port:           30001,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[{"email":"` + backfillEmail + `","id":"` + uid + `","enable":true}]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(clientInbound).Error; err != nil {
		t.Fatalf("create client inbound: %v", err)
	}

	// Inbound B: a legacy MultiDomain inbound whose tag carries the 0.0.0.0: prefix.
	// Its presence makes the externalProxy query return rows, so the function does not
	// early-return and reaches the tag-cleanup statement.
	multiDomainInbound := &model.Inbound{
		UserId:         1,
		Tag:            "inbound-0.0.0.0:30002",
		Enable:         true,
		Port:           30002,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"security":"tls","tlsSettings":{"settings":{"domains":[{"domain":"example.com"}]}}}`,
	}
	if err := db.Create(multiDomainInbound).Error; err != nil {
		t.Fatalf("create multidomain inbound: %v", err)
	}

	var before int64
	if err := db.Model(xray.ClientTraffic{}).Count(&before).Error; err != nil {
		t.Fatalf("count client_traffics before: %v", err)
	}
	if before != 0 {
		t.Fatalf("expected no client_traffics before migration, got %d", before)
	}

	svc := InboundService{}
	svc.MigrationRequirements()

	// The backfill must have committed: the settings-only client now owns a row.
	// Before the fix this was rolled back whenever the externalProxy detection query
	// errored (it does on Postgres via json_extract), so the MultiDomain inbound below
	// is deliberately present to make that query return rows and run to completion.
	var ct xray.ClientTraffic
	if err := db.Model(xray.ClientTraffic{}).Where("email = ?", backfillEmail).First(&ct).Error; err != nil {
		t.Fatalf("client_traffics row not backfilled for %s: %v", backfillEmail, err)
	}

	// The MultiDomain→ExternalProxy migration must have committed too: the detection
	// query ran (.Scan executes it) and the loop rewrote the inbound's streamSettings.
	var refreshed model.Inbound
	if err := db.First(&refreshed, multiDomainInbound.Id).Error; err != nil {
		t.Fatalf("reload multidomain inbound: %v", err)
	}
	if !strings.Contains(refreshed.StreamSettings, "externalProxy") {
		t.Errorf("MultiDomain migration did not commit; streamSettings = %q", refreshed.StreamSettings)
	}
}

func TestMigrationRequirementsReturnsAddClientStatFailure(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })
	db := database.GetDB()
	first := &model.Inbound{UserId: 1, Tag: "first", Port: 31001, Protocol: model.VLESS, Settings: `{"clients":[{"email":"first@example.test","id":"id-1"}]}`, StreamSettings: `{}`}
	if err := db.Create(first).Error; err != nil {
		t.Fatalf("create first: %v", err)
	}
	const injected = "injected AddClientStat failure"
	failSave := func(tx *gorm.DB) {
		tx.AddError(errors.New(injected))
	}
	if err := db.Callback().Update().Before("gorm:update").Register("test:fail-migration-inbound-save", failSave); err != nil {
		t.Fatalf("register update callback: %v", err)
	}
	if err := db.Callback().Create().Before("gorm:create").Register("test:fail-migration-inbound-save", failSave); err != nil {
		t.Fatalf("register create callback: %v", err)
	}
	err := (&InboundService{}).MigrationRequirements()
	if err == nil || err.Error() != injected {
		t.Fatalf("MigrationRequirements error = %v, want %q", err, injected)
	}
	var count int64
	if err := db.Model(&xray.ClientTraffic{}).Where("email = ?", "first@example.test").Count(&count).Error; err != nil {
		t.Fatalf("count rolled-back traffic: %v", err)
	}
	if count != 0 {
		t.Fatalf("earlier traffic write committed after save failure: count=%d", count)
	}
}

// TestMigrationRequirements_CleansLegacyZeroAddrTag guards the legacy tag cleanup that
// strips the auto-generated "0.0.0.0:" prefix. The inbound is MultiDomain TLS so the
// externalProxy detection query returns rows and the cleanup is reached (it early-returns
// at len(externalProxy)==0 otherwise). The cleanup must use tx.Exec, not tx.Raw, which
// only builds a non-SELECT statement without running it.
func TestMigrationRequirements_CleansLegacyZeroAddrTag(t *testing.T) {
	dbDir := t.TempDir()
	t.Setenv("XUI_DB_FOLDER", dbDir)
	if err := database.InitDB(filepath.Join(dbDir, "x-ui.db")); err != nil {
		t.Fatalf("InitDB: %v", err)
	}
	t.Cleanup(func() { _ = database.CloseDB() })

	db := database.GetDB()
	legacy := &model.Inbound{
		UserId:         1,
		Tag:            "inbound-0.0.0.0:30002",
		Enable:         true,
		Port:           30002,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"security":"tls","tlsSettings":{"settings":{"domains":[{"domain":"example.com"}]}}}`,
	}
	if err := db.Create(legacy).Error; err != nil {
		t.Fatalf("create legacy inbound: %v", err)
	}

	svc := InboundService{}
	svc.MigrationRequirements()

	var got model.Inbound
	if err := db.First(&got, legacy.Id).Error; err != nil {
		t.Fatalf("reload inbound: %v", err)
	}
	if got.Tag != "inbound-30002" {
		t.Fatalf("legacy 0.0.0.0: tag not stripped: got %q, want %q", got.Tag, "inbound-30002")
	}
}

func TestMigrationRemoveOrphanedTraffics(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()
	clientSvc := &ClientService{}
	inboundSvc := &InboundService{}

	const attachedEmail = "attached@example.com"
	attachedClient := model.Client{Email: attachedEmail, ID: "11111111-1111-1111-1111-111111111111", SubID: attachedEmail, Enable: true}
	attachedIb := mkInbound(t, 30003, model.VLESS, clientsSettings(t, []model.Client{attachedClient}))
	if err := clientSvc.SyncInbound(nil, attachedIb.Id, []model.Client{attachedClient}); err != nil {
		t.Fatalf("seed attached client: %v", err)
	}
	mkTraffic(t, attachedIb.Id, attachedEmail, 0, 0, 0, 0, true)

	const detachedEmail = "detached@example.com"
	detachedClient := model.Client{Email: detachedEmail, ID: "22222222-2222-2222-2222-222222222222", SubID: detachedEmail, Enable: true}
	detachedIb := mkInbound(t, 30004, model.VLESS, clientsSettings(t, []model.Client{detachedClient}))
	if err := clientSvc.SyncInbound(nil, detachedIb.Id, []model.Client{detachedClient}); err != nil {
		t.Fatalf("seed detached client: %v", err)
	}
	mkTraffic(t, detachedIb.Id, detachedEmail, 123, 456, 0, 0, true)
	detachedRec := lookupClientRecord(t, detachedEmail)
	if _, err := clientSvc.Detach(inboundSvc, detachedRec.Id, []int{detachedIb.Id}); err != nil {
		t.Fatalf("Detach: %v", err)
	}

	const jsonOnlyEmail = "jsononly@example.com"
	jsonOnlyClient := model.Client{Email: jsonOnlyEmail, ID: "33333333-3333-3333-3333-333333333333", SubID: jsonOnlyEmail, Enable: true}
	jsonOnlyIb := mkInbound(t, 30005, model.VLESS, clientsSettings(t, []model.Client{jsonOnlyClient}))
	mkTraffic(t, jsonOnlyIb.Id, jsonOnlyEmail, 0, 0, 0, 0, true)

	const trulyOrphanedEmail = "deleted@example.com"
	mkTraffic(t, attachedIb.Id, trulyOrphanedEmail, 0, 0, 0, 0, true)

	inboundSvc.MigrationRemoveOrphanedTraffics()

	cases := []struct {
		name  string
		email string
		want  int64
	}{
		{"attached, in clients table and JSON", attachedEmail, 1},
		{"detached-but-alive, in clients table only", detachedEmail, 1},
		{"seeder-skipped-but-live, in JSON only", jsonOnlyEmail, 1},
		{"truly orphaned, in neither", trulyOrphanedEmail, 0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got int64
			if err := db.Model(xray.ClientTraffic{}).Where("email = ?", c.email).Count(&got).Error; err != nil {
				t.Fatalf("count client_traffics for %s: %v", c.email, err)
			}
			if got != c.want {
				t.Errorf("client_traffics count for %s: got %d, want %d", c.email, got, c.want)
			}
		})
	}
}

func TestMigrationRequirements_NormalizesShareAddressFields(t *testing.T) {
	setupConflictDB(t)
	db := database.GetDB()

	invalidStrategy := &model.Inbound{
		UserId:         1,
		Tag:            "invalid-share-strategy",
		Enable:         true,
		Port:           31001,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	paddedStrategy := &model.Inbound{
		UserId:         1,
		Tag:            "padded-share-strategy",
		Enable:         true,
		Port:           31002,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	invalidAddress := &model.Inbound{
		UserId:         1,
		Tag:            "invalid-share-address",
		Enable:         true,
		Port:           31003,
		Protocol:       model.VLESS,
		Settings:       `{"clients":[]}`,
		StreamSettings: `{"network":"tcp","security":"none"}`,
	}
	if err := db.Create(invalidStrategy).Error; err != nil {
		t.Fatalf("create invalid strategy inbound: %v", err)
	}
	if err := db.Create(paddedStrategy).Error; err != nil {
		t.Fatalf("create padded strategy inbound: %v", err)
	}
	if err := db.Create(invalidAddress).Error; err != nil {
		t.Fatalf("create invalid address inbound: %v", err)
	}
	if err := db.Model(&model.Inbound{}).Where("id = ?", invalidStrategy.Id).Updates(map[string]any{
		"share_addr_strategy": " auto ",
		"share_addr":          "  edge.example.com  ",
	}).Error; err != nil {
		t.Fatalf("seed invalid share fields: %v", err)
	}
	if err := db.Model(&model.Inbound{}).Where("id = ?", paddedStrategy.Id).Updates(map[string]any{
		"share_addr_strategy": " listen ",
		"share_addr":          "  10.0.0.1  ",
	}).Error; err != nil {
		t.Fatalf("seed padded share fields: %v", err)
	}
	if err := db.Model(&model.Inbound{}).Where("id = ?", invalidAddress.Id).Updates(map[string]any{
		"share_addr_strategy": "custom",
		"share_addr":          "edge.example.com:8443",
	}).Error; err != nil {
		t.Fatalf("seed invalid address share fields: %v", err)
	}

	svc := InboundService{}
	svc.MigrationRequirements()

	var gotInvalid model.Inbound
	if err := db.First(&gotInvalid, invalidStrategy.Id).Error; err != nil {
		t.Fatalf("reload invalid strategy inbound: %v", err)
	}
	if gotInvalid.ShareAddrStrategy != "node" || gotInvalid.ShareAddr != "edge.example.com" {
		t.Fatalf("invalid share fields = (%q, %q), want (node, edge.example.com)", gotInvalid.ShareAddrStrategy, gotInvalid.ShareAddr)
	}

	var gotPadded model.Inbound
	if err := db.First(&gotPadded, paddedStrategy.Id).Error; err != nil {
		t.Fatalf("reload padded strategy inbound: %v", err)
	}
	if gotPadded.ShareAddrStrategy != "listen" || gotPadded.ShareAddr != "10.0.0.1" {
		t.Fatalf("padded share fields = (%q, %q), want (listen, 10.0.0.1)", gotPadded.ShareAddrStrategy, gotPadded.ShareAddr)
	}

	var gotInvalidAddress model.Inbound
	if err := db.First(&gotInvalidAddress, invalidAddress.Id).Error; err != nil {
		t.Fatalf("reload invalid address inbound: %v", err)
	}
	if gotInvalidAddress.ShareAddrStrategy != "node" || gotInvalidAddress.ShareAddr != "" {
		t.Fatalf("invalid address share fields = (%q, %q), want (node, empty)", gotInvalidAddress.ShareAddrStrategy, gotInvalidAddress.ShareAddr)
	}
}

const (
	liveTunnelPriv  = "awg-live-priv"
	liveTunnelPub   = "awg-live-pub"
	liveTunnelPSK   = "awg-live-psk"
	staleTunnelPriv = "vless-copy-priv"
	staleTunnelPub  = "vless-copy-pub"
	staleTunnelPSK  = "vless-copy-psk"
)

// Vision is only restorable on a transport that can carry it; plain is the neutral
// stand-in mkInbound omits, and json_extract rejects that empty column outright.
const (
	plainStream  = `{"network":"tcp","security":"none"}`
	visionStream = `{"network":"tcp","security":"tls"}`
)

// mkMigrationInbound is mkInbound plus stream settings: the externalProxy detection
// query runs json_extract over that column and rejects the empty string mkInbound leaves.
func mkMigrationInbound(t *testing.T, port int, proto model.Protocol, settings, stream string) *model.Inbound {
	t.Helper()
	ib := mkInbound(t, port, proto, settings)
	if err := database.GetDB().Model(&model.Inbound{}).Where("id = ?", ib.Id).
		Update("stream_settings", stream).Error; err != nil {
		t.Fatalf("set stream settings on inbound %d: %v", port, err)
	}
	return ib
}

// seedTunnelKeyClobber builds the shape that broke two live panels: one identity whose
// live keypair comes from an AWG inbound, plus a keyless inbound holding a stale copy.
func seedTunnelKeyClobber(t *testing.T, email string, awgPort, keylessPort int) *model.Inbound {
	t.Helper()
	awgClient := model.Client{
		Email:        email,
		SubID:        "sub-" + email,
		Enable:       true,
		PrivateKey:   liveTunnelPriv,
		PublicKey:    liveTunnelPub,
		PreSharedKey: liveTunnelPSK,
		AllowedIPs:   []string{"10.200.0.2/32"},
	}
	awgIb := mkMigrationInbound(t, awgPort, model.AWG, clientsSettings(t, []model.Client{awgClient}), plainStream)
	if err := (&ClientService{}).SyncInbound(nil, awgIb.Id, []model.Client{awgClient}); err != nil {
		t.Fatalf("seed AWG client record: %v", err)
	}

	stale := awgClient
	stale.ID = "9f1d0f6e-1c2b-4a3d-8e5f-0a1b2c3d4e5f"
	stale.AllowedIPs = nil
	stale.PrivateKey = staleTunnelPriv
	stale.PublicKey = staleTunnelPub
	stale.PreSharedKey = staleTunnelPSK
	mkMigrationInbound(t, keylessPort, model.VLESS, clientsSettings(t, []model.Client{stale}), plainStream)
	return awgIb
}

func assertLiveTunnelKeys(t *testing.T, email, when string) {
	t.Helper()
	rec := lookupClientRecord(t, email)
	if rec.PrivateKey != liveTunnelPriv || rec.PublicKey != liveTunnelPub || rec.PreSharedKey != liveTunnelPSK {
		t.Fatalf("%s: client record keys = (%q, %q, %q), want (%q, %q, %q)",
			when, rec.PrivateKey, rec.PublicKey, rec.PreSharedKey,
			liveTunnelPriv, liveTunnelPub, liveTunnelPSK)
	}
}

// x-ui migrate runs on every web update and syncs only keyless inbounds, so a stale
// tunnel keypair sitting in their settings JSON used to overwrite the working one.
func TestMigrationRequirements_KeylessInboundKeepsTunnelKeys(t *testing.T) {
	setupBulkDB(t)
	const email = "demo-user@example.test"
	seedTunnelKeyClobber(t, email, 32001, 32002)

	if err := (&InboundService{}).MigrationRequirements(); err != nil {
		t.Fatalf("MigrationRequirements: %v", err)
	}
	assertLiveTunnelKeys(t, email, "after migrate")
}

// The record feeds the subscription .conf and the AWG inbound's settings feed the kernel
// peer: a tunnel client's keys are legitimate, and migrate must not drive the two apart.
func TestMigrationRequirements_ClientRecordStillMatchesTunnelPeer(t *testing.T) {
	setupBulkDB(t)
	const email = "demo-pair@example.test"
	awgIb := seedTunnelKeyClobber(t, email, 32021, 32022)

	if err := (&InboundService{}).MigrationRequirements(); err != nil {
		t.Fatalf("MigrationRequirements: %v", err)
	}

	var got model.Inbound
	if err := database.GetDB().First(&got, awgIb.Id).Error; err != nil {
		t.Fatalf("reload AWG inbound: %v", err)
	}
	peers, err := (&InboundService{}).GetClients(&got)
	if err != nil {
		t.Fatalf("parse AWG inbound clients: %v", err)
	}
	var peer model.Client
	for _, p := range peers {
		if p.Email == email {
			peer = p
		}
	}
	if peer.Email == "" {
		t.Fatalf("client %q vanished from the AWG inbound settings: %s", email, got.Settings)
	}

	rec := lookupClientRecord(t, email)
	if rec.PrivateKey != peer.PrivateKey || rec.PublicKey != peer.PublicKey || rec.PreSharedKey != peer.PreSharedKey {
		t.Fatalf("record drifted from the AWG peer: record = (%q, %q, %q), inbound = (%q, %q, %q)",
			rec.PrivateKey, rec.PublicKey, rec.PreSharedKey,
			peer.PrivateKey, peer.PublicKey, peer.PreSharedKey)
	}
}

func TestMigrateDB_TunnelKeysUnchangedOnSecondRun(t *testing.T) {
	setupBulkDB(t)
	const email = "demo-idem@example.test"
	seedTunnelKeyClobber(t, email, 32011, 32012)

	svc := &InboundService{}
	svc.MigrateDB()
	assertLiveTunnelKeys(t, email, "after first migrate")
	first := lookupClientRecord(t, email)

	svc.MigrateDB()
	assertLiveTunnelKeys(t, email, "after second migrate")
	second := lookupClientRecord(t, email)

	// Every run restamps updated_at in the settings JSON by design; nothing else may move.
	first.UpdatedAt = second.UpdatedAt
	if first != second {
		t.Fatalf("second MigrateDB changed the client record:\n first  = %+v\n second = %+v", first, second)
	}
}

// MigrationRestoreVisionFlow is the second migrate path that syncs a keyless inbound's
// settings into the clients table, and it clobbered the tunnel keypair the same way.
func TestMigrationRestoreVisionFlow_KeepsTunnelKeys(t *testing.T) {
	setupBulkDB(t)
	const email = "demo-vision@example.test"
	clientSvc := &ClientService{}

	awgClient := model.Client{
		Email:        email,
		SubID:        "sub-vision",
		Enable:       true,
		PrivateKey:   liveTunnelPriv,
		PublicKey:    liveTunnelPub,
		PreSharedKey: liveTunnelPSK,
		AllowedIPs:   []string{"10.200.0.4/32"},
	}
	awgIb := mkMigrationInbound(t, 32031, model.AWG, clientsSettings(t, []model.Client{awgClient}), plainStream)
	if err := clientSvc.SyncInbound(nil, awgIb.Id, []model.Client{awgClient}); err != nil {
		t.Fatalf("seed AWG client record: %v", err)
	}

	// The sibling link supplies flow_override=vision, which is the intended flow the
	// restore reads back; with no such link it finds nothing to heal and never syncs.
	sibling := model.Client{
		Email: email, ID: "5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f",
		SubID: "sub-vision", Enable: true, Flow: visionFlow,
	}
	siblingIb := mkMigrationInbound(t, 32032, model.VLESS, clientsSettings(t, []model.Client{sibling}), visionStream)
	if err := clientSvc.SyncInbound(nil, siblingIb.Id, []model.Client{sibling}); err != nil {
		t.Fatalf("seed vision flow override: %v", err)
	}

	// The inbound to heal: flow lost, and the stale copy of the tunnel keypair alongside.
	stale := model.Client{
		Email: email, ID: "5c6d7e8f-9a0b-4c1d-8e2f-3a4b5c6d7e8f",
		SubID: "sub-vision", Enable: true,
		PrivateKey: staleTunnelPriv, PublicKey: staleTunnelPub, PreSharedKey: staleTunnelPSK,
	}
	targetIb := mkMigrationInbound(t, 32033, model.VLESS, clientsSettings(t, []model.Client{stale}), visionStream)

	(&InboundService{}).MigrationRestoreVisionFlow()

	var healed model.Inbound
	if err := database.GetDB().First(&healed, targetIb.Id).Error; err != nil {
		t.Fatalf("reload healed inbound: %v", err)
	}
	if !strings.Contains(healed.Settings, visionFlow) {
		t.Fatalf("restore never ran, so the sync under test never happened: settings = %s", healed.Settings)
	}
	assertLiveTunnelKeys(t, email, "after vision flow restore")
}
