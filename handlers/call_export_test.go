package handlers

// SetRoomAPI подменяет клиент LiveKit во внешнем тест-пакете (handlers_test).
// Настоящий ходит по сети, а пробные комнаты после переезда состояния в LiveKit
// без него не проверить. Файл с суффиксом _test.go в прод-сборку не попадает.
func (h *CallHandler) SetRoomAPI(api roomAPI) { h.roomClient = api }
