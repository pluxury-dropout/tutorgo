package handlers

// Экспортируем во внешний тест-пакет (handlers_test) константы, из которых
// выведен wbByteRateLimit. Тест на "легитимный поток" обязан гонять именно
// потолок — wbMaxMessageBytesForTest раз в wbUpdateThrottleMsForTest мс, —
// а не задублированные магические числа, которые могут разъехаться с
// реализацией так же, как уже разъехались один раз (см. комментарий у
// wbByteRateLimit в whiteboard_ws.go).
const (
	WbMaxMessageBytesForTest  = wbMaxMessageBytes
	WbUpdateThrottleMsForTest = wbUpdateThrottleMs
)
