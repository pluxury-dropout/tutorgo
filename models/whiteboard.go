package models

import (
	"encoding/json"
	"errors"
	"time"
)

type Board struct {
	ID        string    `json:"id"`
	CourseID  string    `json:"course_id"`
	TutorID   string    `json:"tutor_id"`
	CreatedAt time.Time `json:"created_at"`
}

type BoardPage struct {
	ID      string `json:"id"`
	BoardID string `json:"board_id"`
	Title   string `json:"title"`
	// Снапшот сюда не кладётся: клиент получает его по WS при подключении к
	// странице, а в списке страниц он раздувал ответ на сотни килобайт.
	Position  int       `json:"position"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BoardAsset struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	FilePath  string    `json:"file_path"`
	MimeType  string    `json:"mime_type"`
	SizeBytes int       `json:"size_bytes"`
	CreatedAt time.Time `json:"created_at"`
}

type BoardInvite struct {
	ID        string    `json:"id"`
	BoardID   string    `json:"board_id"`
	CreatedAt time.Time `json:"created_at"`
}

type CreateBoardPageRequest struct {
	Title string `json:"title" validate:"required,min=1,max=100"`
}

type UpdateBoardPageRequest struct {
	Title    *string `json:"title"    validate:"omitempty,min=1,max=100"`
	Position *int    `json:"position" validate:"omitempty,min=0"`
}

type BoardWithPages struct {
	Board
	Pages []BoardPage `json:"pages"`
}

type BoardAssetResponse struct {
	ID  string `json:"id"`
	URL string `json:"url"`
}

// BoardElement — один элемент сцены Excalidraw в поэлементной модели хранения.
// Data — элемент целиком, как его прислал клиент; version/nonce вытащены из
// него же, потому что по ним идёт слияние.
type BoardElement struct {
	ID      string
	Version int
	Nonce   int64
	Data    json.RawMessage
}

// Beats сообщает, вытесняет ли e элемент other.
//
// Правило — то же, по которому сливает клиентский reconcileElements Excalidraw:
// побеждает больший version, при равных версиях — МЕНЬШИЙ versionNonce
// (`shouldDiscardRemoteElement`: «resolve conflicting edits deterministically by
// taking the one with the lowest versionNonce»). Интуиция подсказывает обратное;
// ошибка здесь даёт молчаливое расхождение сервера и клиентов.
//
// Дубликат этого правила живёт в SQL-клаузе mergeElementSQL — там оно
// авторитетно, здесь нужно, чтобы буфер не терял победителя, схлопывая правки
// одного элемента до похода в БД. Расходиться они не должны: тесты гоняют
// одну таблицу случаев.
func (e BoardElement) Beats(other BoardElement) bool {
	if e.Version != other.Version {
		return e.Version > other.Version
	}
	return e.Nonce < other.Nonce
}

// maxElementsPerMessage — потолок на число элементов в одном сообщении.
// Вставка PDF на 100 страниц — крупнейший честный пакет, что мы видели;
// запас на порядок.
const maxElementsPerMessage = 5000

var errTooManyElements = errors.New("too many elements in message")

// ParseBoardElements разбирает элементы из сообщения клиента, отбрасывая
// невалидные. Граница доверия сместилась вместе с ролью сервера: он перешёл из
// ретранслятора в хранилище, и «мусор» тут — это мусор в БД.
//
// Возвращает ошибку только на переполнение: отдельные битые элементы молча
// пропускаются, потому что рвать из-за них весь пакет — значит терять и
// соседние валидные штрихи.
func ParseBoardElements(raw []json.RawMessage) ([]BoardElement, error) {
	if len(raw) > maxElementsPerMessage {
		return nil, errTooManyElements
	}
	out := make([]BoardElement, 0, len(raw))
	for _, item := range raw {
		var head struct {
			ID           string `json:"id"`
			Version      *int   `json:"version"`
			VersionNonce *int64 `json:"versionNonce"`
		}
		if err := json.Unmarshal(item, &head); err != nil {
			continue
		}
		// version — указатель, чтобы отличить отсутствующее поле от нуля:
		// элемент без version сливать не по чему, он бы вечно проигрывал.
		if head.ID == "" || head.Version == nil || *head.Version < 0 || head.VersionNonce == nil {
			continue
		}
		out = append(out, BoardElement{
			ID:      head.ID,
			Version: *head.Version,
			Nonce:   *head.VersionNonce,
			Data:    item,
		})
	}
	return out, nil
}
