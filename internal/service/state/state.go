package state

import "sync"

// State — потокобезопасный буфер фиксированной ёмкости для сообщений из игрового состояния.
type State struct {
	cap      int
	messages []string
	mu       sync.Mutex
	notify   chan struct{}
}

func New(capacity int) *State {
	if capacity <= 0 {
		capacity = 20
	}
	return &State{cap: capacity, messages: make([]string, 0, capacity), notify: make(chan struct{}, 1)}
}

// Add добавляет сообщение, при переполнении удаляет самое старое.
func (s *State) Add(text string) {
	if text == "" {
		return
	}
	s.mu.Lock()
	if len(s.messages) == s.cap {
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
func (s *State) Drain() []string {
	s.mu.Lock()
	msgs := make([]string, len(s.messages))
	copy(msgs, s.messages)
	s.messages = s.messages[:0]
	s.mu.Unlock()
	return msgs
}

func (s *State) Len() int {
	s.mu.Lock()
	l := len(s.messages)
	s.mu.Unlock()
	return l
}

func (s *State) NotifyCh() <-chan struct{} { return s.notify }
