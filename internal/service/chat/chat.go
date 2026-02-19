package chat

import "sync"

// Chat — потокобезопасный буфер фиксированной ёмкости для сообщений из чата.
type Chat struct {
	cap      int
	messages []string
	mu       sync.Mutex
}

func New(capacity int) *Chat {
	if capacity <= 0 {
		capacity = 30
	}
	return &Chat{cap: capacity, messages: make([]string, 0, capacity)}
}

// Add добавляет сообщение, при переполнении удаляет самое старое.
func (c *Chat) Add(text string) {
	if text == "" {
		return
	}
	c.mu.Lock()
	if len(c.messages) == c.cap {
		// удалить самое старое
		copy(c.messages, c.messages[1:])
		c.messages = c.messages[:c.cap-1]
	}
	c.messages = append(c.messages, text)
	c.mu.Unlock()
}

// Drain возвращает все сообщения и очищает буфер.
func (c *Chat) Drain() []string {
	c.mu.Lock()
	msgs := make([]string, len(c.messages))
	copy(msgs, c.messages)
	c.messages = c.messages[:0]
	c.mu.Unlock()
	return msgs
}

func (c *Chat) Len() int {
	c.mu.Lock()
	l := len(c.messages)
	c.mu.Unlock()
	return l
}
