package main

import (
	"encoding/json"
	"testing"
)

func TestCacheUpsertGet(t *testing.T) {
	cache = make(map[string]json.RawMessage)
	id := "test1"
	raw := json.RawMessage(`{"order_uid":"test1","foo":"bar"}`)
	upsertCache(id, raw)
	got, ok := getFromCache(id)
	if !ok {
		t.Fatal("expected found in cache")
	}
	if string(got) != string(raw) {
		t.Fatalf("mismatch: %s != %s", string(got), string(raw))
	}
}
