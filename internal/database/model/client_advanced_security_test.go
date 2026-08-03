package model

import (
	"encoding/json"
	"testing"
)

// TestToRecordToClient_AdvancedSecurityRoundTrip verifies the AWG3
// AdvancedSecurity flag survives the Client→ClientRecord→Client round-trip
// (the gorm-backed cache table). Before lucx.54 the field was absent from
// both structs, so the Switch toggled in the UI silently reverted to OFF
// after save — the operator's choice was dropped on the json.Unmarshal into
// model.Client and never reached the ClientRecord column.
func TestToRecordToClient_AdvancedSecurityRoundTrip(t *testing.T) {
	c := &Client{
		Email:            "advanced@example.com",
		Enable:           true,
		AdvancedSecurity: true,
	}

	rec := c.ToRecord()
	if !rec.AdvancedSecurity {
		t.Fatal("ToRecord: AdvancedSecurity lost (Client→ClientRecord)")
	}

	back := rec.ToClient()
	if !back.AdvancedSecurity {
		t.Fatal("ToClient: AdvancedSecurity lost (ClientRecord→Client)")
	}
}

// TestClientJSONUnmarshal_AdvancedSecurityCaptured confirms the struct tag
// captures the field from a client payload — the exact path the frontend
// takes when saving a client (controller binds into model.Client).
func TestClientJSONUnmarshal_AdvancedSecurityCaptured(t *testing.T) {
	payload := `{"email":"x@example.com","enable":true,"advancedSecurity":true}`
	var c Client
	if err := json.Unmarshal([]byte(payload), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !c.AdvancedSecurity {
		t.Fatal("AdvancedSecurity not captured from JSON payload")
	}
}

// TestClientJSONUnmarshal_AdvancedSecurityDefaultsFalse verifies the zero
// value is false when the field is absent (non-v3 clients, older payloads).
func TestClientJSONUnmarshal_AdvancedSecurityDefaultsFalse(t *testing.T) {
	payload := `{"email":"y@example.com","enable":true}`
	var c Client
	if err := json.Unmarshal([]byte(payload), &c); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if c.AdvancedSecurity {
		t.Fatal("AdvancedSecurity defaulted to true, want false")
	}
}

// TestClientRecordMarshal_AdvancedSecurityInJSON confirms the field appears
// in the serialized ClientRecord JSON (the API response shape the frontend
// re-seeds the form from).
func TestClientRecordMarshal_AdvancedSecurityInJSON(t *testing.T) {
	rec := ClientRecord{
		Email:            "z@example.com",
		Enable:           true,
		AdvancedSecurity: true,
	}
	out, err := json.Marshal(rec)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var probe map[string]any
	if err := json.Unmarshal(out, &probe); err != nil {
		t.Fatalf("re-unmarshal: %v", err)
	}
	if v, ok := probe["advancedSecurity"]; !ok || v != true {
		t.Fatalf("advancedSecurity missing or wrong in ClientRecord JSON: got %v (ok=%v)", v, ok)
	}
}
