package localconversation

// LocalConversation имитирует сущность диалога на стороне приложения.
type LocalConversation struct {
	ID              string
	ResponseHistory []string
	maxRecords      int
}

// New создаёт новый локальный диалог с ограничением на размер истории.
func New(id string, maxRecords int) *LocalConversation {
	if maxRecords < 0 {
		maxRecords = 0
	}
	return &LocalConversation{ID: id, ResponseHistory: make([]string, 0, maxRecords), maxRecords: maxRecords}
}

// AppendResponse добавляет ответ ИИ в локальную историю.
func (lc *LocalConversation) AppendResponse(resp string) {
	lc.ResponseHistory = append(lc.ResponseHistory, resp)
	if lc.maxRecords > 0 && len(lc.ResponseHistory) > lc.maxRecords {
		// Оставляем последние maxRecords элементов
		lc.ResponseHistory = lc.ResponseHistory[len(lc.ResponseHistory)-lc.maxRecords:]
	}
}

// History возвращает срез ответов как есть.
func (lc *LocalConversation) History() []string {
	return lc.ResponseHistory
}
