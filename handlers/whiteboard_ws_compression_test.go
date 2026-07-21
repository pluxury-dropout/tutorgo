package handlers_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"tutorgo/handlers"
	"tutorgo/models"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/mock"
)

// dialBoardWS поднимает httptest-сервер с mgr.ServeWS (как в router.go) и
// подключается настоящим WS-клиентом с EnableCompression:true — тем же
// путём, которым реально пойдёт атакующий с сырым WS-клиентом (браузерный JS
// не даёт собрать произвольный сжатый фрейм руками, raw-клиент — легко).
//
// M-2 из ревью: dialer.Dial молча негоциирует расширение, и если сервер по
// какой-то причине его не предложит (например, кто-то уберёт
// EnableCompression на Upgrader), тесты этого файла продолжат слать через
// compression-клиент и упадут на первом чтении с невнятным "connection reset
// by peer" — флак-читаемая ошибка вместо явного сигнала о причине. Проверяем
// Sec-WebSocket-Extensions сразу и явно.
func dialBoardWS(t *testing.T, pageID, boardID, inviteToken string) (conn *websocket.Conn, closeServer func()) {
	t.Helper()
	svc := new(mockWhiteboardService)
	subs := new(mockSubscriptionService)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	mgr := handlers.NewWbHubManager(svc, subs, log, "test-secret", nil)

	svc.On("ValidateInvite", mock.Anything, inviteToken).Return(models.BoardWithPages{
		Board: models.Board{ID: boardID},
		Pages: []models.BoardPage{{ID: pageID, BoardID: boardID}},
	}, nil)
	svc.On("GetPageState", mock.Anything, pageID).Return(json.RawMessage(`{"elements":[],"files":{}}`), nil)

	r := gin.New()
	r.GET("/ws/board/:pageId", mgr.ServeWS(svc))
	ts := httptest.NewServer(r)

	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/ws/board/" + pageID + "?token=" + inviteToken
	dialer := websocket.Dialer{EnableCompression: true}
	c, resp, err := dialer.Dial(wsURL, nil)
	if err != nil {
		ts.Close()
		t.Fatalf("dial: %v", err)
	}
	if ext := resp.Header.Get("Sec-WebSocket-Extensions"); !strings.Contains(ext, "permessage-deflate") {
		c.Close()
		ts.Close()
		t.Fatalf("сервер не согласовал permessage-deflate, Sec-WebSocket-Extensions=%q", ext)
	}

	return c, ts.Close
}

// readSeed вычитывает первый (сид) кадр, который сервер шлёт сразу после
// апгрейда, — он не должен мешать проверкам ниже.
func readSeed(t *testing.T, conn *websocket.Conn) {
	t.Helper()
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	if _, _, err := conn.ReadMessage(); err != nil {
		t.Fatalf("read seed: %v", err)
	}
}

// Гвард в readPump (wbMaxMessageBytes в whiteboard_ws.go) — единственное, что
// стоит между включённым permessage-deflate и decompression bomb (CWE-409):
// deflate жмёт сильно повторяющиеся данные с коэффициентом до ~1000:1, а
// SetReadLimit после включения сжатия бьёт только по байтам НА ПРОВОДЕ — сам
// распакованный поток без ручного io.LimitReader поверх NextReader ничем не
// ограничен. Тест шлёт сообщение, которое укладывается в лимит на проводе, но
// разжимается в разы больше wbMaxMessageBytes, и проверяет, что сервер рвёт
// соединение close-кодом 1009, а не молча съедает память.
//
// Проверено вручную (не автоматизировано): если вернуть тело цикла в readPump
// к плоскому c.conn.ReadMessage() без LimitReader, этот тест падает — второе
// чтение не получает CloseError, а упирается в SetReadDeadline.
func TestBoardWSDecompressionBombClosesConnection(t *testing.T) {
	conn, closeServer := dialBoardWS(t,
		"99999999-9999-9999-9999-999999999999",
		"aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		"bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb")
	defer closeServer()
	defer conn.Close()
	readSeed(t, conn)

	// 2 МБ одного байта — deflate жмёт такое в килобайты на проводе, но
	// распакуется обратно в те же 2 МБ, что кратно больше wbMaxMessageBytes
	// (512 КБ).
	bomb := bytes.Repeat([]byte{'A'}, 2*1024*1024)
	if err := conn.WriteMessage(websocket.TextMessage, bomb); err != nil {
		t.Fatalf("write bomb: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("ожидали, что сервер закроет соединение после decompression bomb, но чтение прошло успешно")
	}
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		t.Fatalf("ожидали *websocket.CloseError (сервер обязан закрыть явно), получили: %v", err)
	}
	if closeErr.Code != websocket.CloseMessageTooBig {
		t.Fatalf("неверный close-код: got %d, want %d (CloseMessageTooBig)", closeErr.Code, websocket.CloseMessageTooBig)
	}
}

// Гард на размер одного сообщения (тест выше) не видит поток из МНОГИХ
// сообщений честного размера, каждое из которых меньше wbMaxMessageBytes, но
// суммарно распаковывающихся на сервере на порядки быстрее, чем клиент
// реально отправил байт на проводе (I-1 из ревью). Байт-рейт-лимитер
// (wbByteRateLimit/wbByteRateBurst) — вторая линия обороны именно на этот
// случай. Тест шлёт сообщения ниже размера-предела, но достаточно часто и
// крупно, чтобы суммарно выйти за burst+пополнение лимитера, и проверяет
// закрытие соединение кодом 1008 (ClosePolicyViolation).
//
// Проверено вручную (не автоматизировано): при временно отключённой проверке
// `c.limiter.AllowN(...)` в readPump этот тест падает — сервер принимает
// весь поток, второе чтение упирается в SetReadDeadline вместо CloseError.
func TestBoardWSByteRateLimitClosesConnection(t *testing.T) {
	conn, closeServer := dialBoardWS(t,
		"11111111-2222-3333-4444-555555555555",
		"66666666-7777-8888-9999-000000000000",
		"cccccccc-cccc-cccc-cccc-cccccccccccc")
	defer closeServer()
	defer conn.Close()
	readSeed(t, conn)

	// 500 КБ — заведомо меньше wbMaxMessageBytes (512 КБ), чтобы сработал
	// именно байт-рейт-лимитер, а не гард на размер одного сообщения. Двух
	// таких сообщений уже почти хватает, чтобы вычерпать burst
	// (2×wbMaxMessageBytes ≈ 1 МБ) — берём с запасом (6 попыток), но крупным
	// шагом, чтобы пополнение бакета между двумя сообщениями (доли
	// миллисекунды на локальном сокете) не успело замаскировать превышение.
	chunk := bytes.Repeat([]byte{'B'}, 500*1024)
	for i := 0; i < 6; i++ {
		if err := conn.WriteMessage(websocket.TextMessage, chunk); err != nil {
			break // сервер мог закрыть соединение уже здесь — не ошибка теста
		}
	}

	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_, _, err := conn.ReadMessage()
	if err == nil {
		t.Fatal("ожидали, что сервер закроет соединение после потока, превышающего byte-rate limit")
	}
	var closeErr *websocket.CloseError
	if !errors.As(err, &closeErr) {
		t.Fatalf("ожидали *websocket.CloseError (byte-rate limit), получили: %v", err)
	}
	if closeErr.Code != websocket.ClosePolicyViolation {
		t.Fatalf("неверный close-код: got %d, want %d (ClosePolicyViolation)", closeErr.Code, websocket.ClosePolicyViolation)
	}
}

// Обратная сторона предыдущего теста: легитимный темп рисования лимит не
// должен задевать — иначе первый же длинный штрих у препода отвалится в
// проде.
//
// C-1 из повторного ревью: первая версия этого теста гоняла "типичный" кадр
// (100 КБ раз в 100 мс ≈ 1 МБ/с), а не потолок, который система УЖЕ
// разрешает — wbMaxMessageBytes (512 КБ) раз в wbUpdateThrottleMs (100 мс,
// т.е. 5.24 МБ/с). Тест с "типичным" кадром прошёл бы и на неверном лимите
// (4 МБ/с), который рвал ровно этот потолок — например, драг крупного
// выделения, гоняющий flushUpdate с диффом всех выделенных элементов разом
// (ревьюер подтвердил эмпирически: 500 КБ раз в 100 мс убивали соединение
// за ~0.91с при лимите 4 МБ/с). Тест обязан гонять именно потолок:
// максимально допустимое сообщение с максимально допустимой частотой.
//
// Размер и период берём не задублированными числами, а из
// whiteboard_ws_export_test.go (WbMaxMessageBytesForTest/
// WbUpdateThrottleMsForTest — реэкспорт wbMaxMessageBytes/
// wbUpdateThrottleMs), чтобы тест не мог разъехаться с реализацией так же,
// как разъехался лимит в C-1: сообщение — РОВНО на пределе wbMaxMessageBytes
// (гард размера пропускает "= предел", режет только "> предел"), интервал —
// ровно wbUpdateThrottleMs. 15 сообщений — дольше, чем упомянутые ~0.91с
// отказа на прежнем (неверном) лимите, так что мутация ниже гарантированно
// ловится, а не проскакивает по границе.
//
// Проверено вручную (не автоматизировано), обе мутации:
//  1. Временно вернул wbByteRateLimit к старому 4*1024*1024 (без формулы) —
//     тест падает: соединение закрывается ClosePolicyViolation до 15-го
//     сообщения.
//  2. Временно отключил проверку c.limiter.AllowN(...) в readPump — тест
//     остаётся зелёным (ожидаемо: без лимитера легитимный поток и подавно не
//     режется), а падает вместо этого TestBoardWSByteRateLimitClosesConnection
//     (см. её комментарий) — так оба гарда покрыты мутациями порознь.
func TestBoardWSNormalDrawingPaceNotRateLimited(t *testing.T) {
	conn, closeServer := dialBoardWS(t,
		"aaaa1111-bbbb-2222-cccc-333344445555",
		"dddd6666-eeee-7777-ffff-888899990000",
		"12121212-3434-3434-5656-565656565656")
	defer closeServer()
	defer conn.Close()
	readSeed(t, conn)

	frame := bytes.Repeat([]byte{'C'}, handlers.WbMaxMessageBytesForTest)
	period := time.Duration(handlers.WbUpdateThrottleMsForTest) * time.Millisecond
	for i := 0; i < 15; i++ {
		if err := conn.WriteMessage(websocket.TextMessage, frame); err != nil {
			t.Fatalf("write frame %d: %v", i, err)
		}
		time.Sleep(period)
	}

	// Второй клиент к этому хабу не подключён, значит вернуться может только
	// одно из двух: таймаут чтения (сервер жив, слать в ответ нечего — это
	// ожидаемый исход) или close-фрейм (сервер счёл легитимный поток
	// превышением — это провал теста).
	conn.SetReadDeadline(time.Now().Add(1 * time.Second))
	_, _, err := conn.ReadMessage()
	var closeErr *websocket.CloseError
	if errors.As(err, &closeErr) {
		t.Fatalf("сервер закрыл соединение на легитимном (предельном) темпе рисования: code=%d", closeErr.Code)
	}
	var netErr net.Error
	if !errors.As(err, &netErr) || !netErr.Timeout() {
		t.Fatalf("ожидали таймаут чтения (соединение живо, слать некому), получили: %v", err)
	}
}
