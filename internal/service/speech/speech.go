package speech

import (
	"sync"
)

// Speech — потокобезопасный буфер фиксированной ёмкости для сообщений из STT.
type Speech struct {
	cap      int
	messages []string
	mu       sync.Mutex
	notify   chan struct{}
}

func New(capacity int) *Speech {
	if capacity <= 0 {
		capacity = 10
	}
	return &Speech{cap: capacity, messages: make([]string, 0, capacity), notify: make(chan struct{}, 1)}
}

// Add добавляет сообщение, при переполнении удаляет самое старое.
func (s *Speech) Add(text string) {
	if text == "" {
		return
	}
	s.mu.Lock()
	if len(s.messages) == s.cap {
		// удалить самое старое
		copy(s.messages, s.messages[1:])
		s.messages = s.messages[:s.cap-1]
	}
	s.messages = append(s.messages, text)
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default:
	}
}

// Drain возвращает все сообщения и очищает буфер.
func (s *Speech) Drain() []string {
	s.mu.Lock()
	msgs := make([]string, len(s.messages))
	copy(msgs, s.messages)
	s.messages = s.messages[:0]
	s.mu.Unlock()
	return msgs
}

func (s *Speech) Len() int {
	s.mu.Lock()
	l := len(s.messages)
	s.mu.Unlock()
	return l
}

func (s *Speech) NotifyCh() <-chan struct{} { return s.notify }
