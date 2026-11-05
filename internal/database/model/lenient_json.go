// Copyright (c) 2026 LucX-UI Project.
// Licensed under the PolyForm Noncommercial License 1.0.0.
// LucX-UI Component. Free for personal and educational use.
// Commercial use (including VPN resale) requires explicit written permission from the author.
// SPDX-License-Identifier: PolyForm-Noncommercial-1.0.0

package model

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm/schema"

	"github.com/mhsanaei/3x-ui/v3/internal/logger"
)

// lenientJSONSerializer reads a JSON column without failing the whole query on
// one malformed row. GORM's stock "json" serializer returns an error from Scan,
// which aborts Find for every row — a single hand-edited Host with alpn
// "h2,http/1.1" made each subscription request answer 500. Here a bad value is
// logged; a []string field falls back to splitting it on commas, anything else
// is left empty. Writes are identical to the stock serializer.
type lenientJSONSerializer struct{}

func init() {
	schema.RegisterSerializer("jsonlenient", lenientJSONSerializer{})
}

func (lenientJSONSerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, dbValue any) error {
	fieldValue := reflect.New(field.FieldType)
	var raw []byte
	switch v := dbValue.(type) {
	case nil:
	case []byte:
		raw = v
	case string:
		raw = []byte(v)
	default:
		raw = []byte(fmt.Sprint(v))
	}
	if text := strings.TrimSpace(string(raw)); text != "" {
		if err := json.Unmarshal(raw, fieldValue.Interface()); err != nil {
			if !lenientStringList(fieldValue, text) {
				fieldValue = reflect.New(field.FieldType)
			}
			logger.Warningf("db: %s.%s holds invalid JSON %q (%v); read leniently",
				field.Schema.Table, field.DBName, truncateForLog(text, 80), err)
		}
	}
	field.ReflectValueOf(ctx, dst).Set(fieldValue.Elem())
	return nil
}

func (lenientJSONSerializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, fieldValue any) (any, error) {
	return schema.JSONSerializer{}.Value(ctx, field, dst, fieldValue)
}

// lenientStringList recovers a []string field stored as "a,b" instead of JSON.
func lenientStringList(ptr reflect.Value, text string) bool {
	if ptr.Elem().Type() != reflect.TypeOf([]string(nil)) {
		return false
	}
	var out []string
	for _, part := range strings.Split(text, ",") {
		if p := strings.Trim(strings.TrimSpace(part), `"[]`); p != "" {
			out = append(out, p)
		}
	}
	ptr.Elem().Set(reflect.ValueOf(out))
	return true
}

func truncateForLog(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
