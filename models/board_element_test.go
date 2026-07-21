package models_test

import (
	"encoding/json"
	"testing"

	"tutorgo/models"
)

func el(version int, nonce int64) models.BoardElement {
	return models.BoardElement{ID: "x", Version: version, Nonce: nonce}
}

// Правило слияния — единственное место, где расхождение сервера с клиентом даёт
// неотлаживаемые баги: доска у двоих разъезжается, и никакой лог этого не
// показывает. Четыре случая — из спеки, они же в mergeElementSQL.
//
// Тай-брейк по МЕНЬШЕМУ nonce взят из исходников Excalidraw
// (shouldDiscardRemoteElement). Интуиция подсказывает обратное — отсюда тест.
func TestBoardElementBeats(t *testing.T) {
	cases := []struct {
		name          string
		incoming, has models.BoardElement
		want          bool
	}{
		{"version больше — принять", el(2, 500), el(1, 500), true},
		{"version меньше — отклонить", el(1, 500), el(2, 500), false},
		{"version равна, nonce меньше — принять", el(2, 100), el(2, 500), true},
		{"version равна, nonce больше — отклонить", el(2, 900), el(2, 500), false},
		{"полное совпадение — не переписывать", el(2, 500), el(2, 500), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.incoming.Beats(tc.has); got != tc.want {
				t.Fatalf("Beats = %v, want %v", got, tc.want)
			}
		})
	}
}

// Слияние обязано быть детерминированным при любом порядке прихода: два
// инстанса пишут один элемент одновременно, и «кто последний» решать нельзя.
func TestBoardElementBeatsAntisymmetric(t *testing.T) {
	pairs := [][2]models.BoardElement{
		{el(1, 500), el(2, 500)},
		{el(2, 100), el(2, 900)},
		{el(3, 1), el(1, 3)},
	}
	for _, p := range pairs {
		if p[0].Beats(p[1]) == p[1].Beats(p[0]) {
			t.Fatalf("оба или ни один не побеждают: %+v vs %+v", p[0], p[1])
		}
	}
}

func TestParseBoardElements(t *testing.T) {
	raw := []json.RawMessage{
		json.RawMessage(`{"id":"ok","version":3,"versionNonce":42,"x":1.5}`),
		json.RawMessage(`{"id":"","version":3,"versionNonce":42}`),    // пустой id
		json.RawMessage(`{"version":3,"versionNonce":42}`),            // нет id
		json.RawMessage(`{"id":"a","versionNonce":42}`),               // нет version
		json.RawMessage(`{"id":"b","version":-1,"versionNonce":42}`),  // version < 0
		json.RawMessage(`{"id":"c","version":3}`),                     // нет versionNonce
		json.RawMessage(`{"id":"d","version":0,"versionNonce":0}`),    // нули — валидны
		json.RawMessage(`не json`),                                    //
		json.RawMessage(`{"id":"e","version":"3","versionNonce":42}`), // version строкой
	}
	got, err := models.ParseBoardElements(raw)
	if err != nil {
		t.Fatalf("неожиданная ошибка: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("прошло %d элементов, ожидалось 2: %+v", len(got), got)
	}
	if got[0].ID != "ok" || got[0].Version != 3 || got[0].Nonce != 42 {
		t.Fatalf("поля разобраны неверно: %+v", got[0])
	}
	// Data — элемент целиком, а не пересобранный: клиенту уедет то же, что пришло.
	if string(got[0].Data) != string(raw[0]) {
		t.Fatalf("Data искажена: %s", got[0].Data)
	}
	if got[1].ID != "d" {
		t.Fatalf("элемент с нулевыми version/nonce отброшен")
	}
}

func TestParseBoardElementsTooMany(t *testing.T) {
	raw := make([]json.RawMessage, 5001)
	for i := range raw {
		raw[i] = json.RawMessage(`{"id":"a","version":1,"versionNonce":1}`)
	}
	if _, err := models.ParseBoardElements(raw); err == nil {
		t.Fatal("переполнение не отвергнуто")
	}
}
