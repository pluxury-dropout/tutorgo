package handlers

import (
	"encoding/json"
	"testing"
)

// Сид — единственный сигнал, по которому клиент разрешает себе персист. Пустая
// страница обязана его получать, битая запись — нет: молчание удерживает клиент
// от записи пустой сцены поверх целой доски.
func TestSeedMessage(t *testing.T) {
	cases := []struct {
		name     string
		snapshot json.RawMessage
		want     string
		wantErr  bool
	}{
		{"пустая страница", nil, `{"type":"snapshot","payload":{"elements":[]}}`, false},
		{"нулевая длина", json.RawMessage{}, `{"type":"snapshot","payload":{"elements":[]}}`, false},
		{"есть снапшот", json.RawMessage(`{"elements":[{"id":"a"}]}`), `{"type":"snapshot","payload":{"elements":[{"id":"a"}]}}`, false},
		{"битая запись", json.RawMessage("not json"), "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := seedMessage(tc.snapshot)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ожидалась ошибка, получено %s", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("неожиданная ошибка: %v", err)
			}
			if string(got) != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
