package repository

import (
	"encoding/json"
	"testing"
)

// Миграция 028 оставила старые снапшоты сырым JSON в bytea, новые пишутся gzip-ом.
// Читатель обязан понимать оба, иначе доски, не открывавшиеся после миграции,
// молча превратятся в пустые.
func TestGunzipSnapshot_ReadsBothFormats(t *testing.T) {
	original := json.RawMessage(`{"elements":[{"id":"a","version":1}],"files":{}}`)

	compressed, err := gzipSnapshot(original)
	if err != nil {
		t.Fatalf("gzipSnapshot: %v", err)
	}
	if !isGzip(compressed) {
		t.Fatal("сжатый снапшот не опознаётся по magic-байтам")
	}

	for _, tc := range []struct {
		name   string
		stored []byte
	}{
		{"новый формат (gzip)", compressed},
		{"наследие миграции (сырой JSON)", original},
	} {
		got, err := gunzipSnapshot(tc.stored)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if string(got) != string(original) {
			t.Errorf("%s: получили %s, ждали %s", tc.name, got, original)
		}
	}

	// NULL-снапшот (страница, на которой ещё не рисовали) не должен падать.
	got, err := gunzipSnapshot(nil)
	if err != nil || got != nil {
		t.Errorf("nil-снапшот: got=%v err=%v", got, err)
	}
}
