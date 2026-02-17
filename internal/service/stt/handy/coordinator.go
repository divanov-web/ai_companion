package handy

import (
	"context"
	"time"
)

type coordinator struct {
	cfg Config

	// входящие от платформенных слушателей
	clipIn   chan Event
	hotkeyIn chan Event

	// исходящие для потребителей
	out chan Event

	// состояние
	lastText   string
	lastTextAt time.Time

	// завершение
	shutdownReady chan struct{}
}

func (c *coordinator) Events() <-chan Event { return c.out }

func (c *coordinator) Run(ctx context.Context) error {
	if c.cfg.HandyWindow <= 0 {
		c.cfg.HandyWindow = time.Second
	}
	if c.cfg.HotkeyDelay < 0 {
		c.cfg.HotkeyDelay = 0
	}

	// стартуем платформенный слушатель (Windows)
	wl, err := newWinListener()
	if err != nil {
		return err
	}

	// запускаем listener в отдельной горутине
	go wl.run(ctx, c.clipIn, c.hotkeyIn)

	// основной цикл координации
	defer close(c.out)

	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case ev := <-c.clipIn:
			// Ретранслируем событие буфера ТОЛЬКО если текст изменился
			if ev.Text != c.lastText {
				c.lastText = ev.Text
				c.lastTextAt = ev.At
				c.safeSend(ev)
			}
		case ev := <-c.hotkeyIn:
			// публикуем «сырое» событие Hotkey
			c.safeSend(ev)
			hotkeyAt := ev.At
			// Отложенная реакция: ждём HotkeyDelay и берём «устоявшийся» текст в буфере
			go func(hkAt time.Time) {
				if c.cfg.HotkeyDelay > 0 {
					t := time.NewTimer(c.cfg.HotkeyDelay)
					defer t.Stop()
					select {
					case <-ctx.Done():
						return
					case <-t.C:
					}
				}
				// Берём самый свежий текст из известного состояния.
				// Проверим, что по времени он недалёк от хоткея (до/после) в пределах окна+задержки.
				lastAt := c.lastTextAt
				txt := c.lastText
				if txt == "" || lastAt.IsZero() {
					return
				}
				d := hkAt.Sub(lastAt)
				if d < 0 {
					d = -d
				}
				limit := c.cfg.HandyWindow + c.cfg.HotkeyDelay
				if d < limit {
					c.safeSend(Event{Type: EventHandyFinalText, Text: txt, At: time.Now()})
				}
			}(hotkeyAt)
		}
	}
}

func (c *coordinator) safeSend(ev Event) {
	select {
	case c.out <- ev:
	default:
		// в случае переполнения — дроп, чтобы не блокировать
	}
}

// Реализация под Windows в файле windows_listener_windows.go
type winListener interface {
	run(ctx context.Context, clipOut chan<- Event, hotkeyOut chan<- Event)
}
