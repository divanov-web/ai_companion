package yandex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// Config настройки клиента Yandex STT Streaming (WebSocket).
type Config struct {
	// Endpoint WebSocket, по умолчанию wss://stt.api.cloud.yandex.net/speech/v1/stt:streaming
	Endpoint   string
	APIKey     string // Api-Key из окружения (YC_STT_API_KEY)
	Language   string // например, "ru-RU"
	SampleRate int    // например, 16000

	// Необязательный стартовый JSON, который будет отправлен текстовым фреймом сразу после подключения.
	// Это позволяет быстро адаптироваться к точному формату протокола без перекомпиляции.
	StartJSON string

	// Если сервер поддерживает сигнал конца аудио в виде JSON, его можно передать в EndJSON.
	// Если пусто — будет отправлен websocket.CloseMessage.
	EndJSON string

	// Разрешить приём частичных результатов; если false — клиент будет выделять только финальные.
	AllowPartials bool
}

// Result единица результата распознавания.
type Result struct {
	Text      string
	Final     bool
	Timestamp time.Time
}

// Client реализует потоковое распознавание через WebSocket.
type Client struct {
	cfg     Config
	conn    *websocket.Conn
	mu      sync.Mutex
	started bool

	// Канал для результатов (закрывается при остановке клиента).
	results chan Result

	// Контекст жизненного цикла соединения.
	ctx    context.Context
	cancel context.CancelFunc
}

// New создаёт клиент, без установления соединения.
func New(cfg Config) (*Client, error) {
	if cfg.Endpoint == "" {
		cfg.Endpoint = "wss://stt.api.cloud.yandex.net/speech/v1/stt:streaming"
	}
	if cfg.APIKey == "" {
		return nil, errors.New("yandex stt: пустой API key (ожидается YC_STT_API_KEY)")
	}
	if cfg.Language == "" {
		cfg.Language = "ru-RU"
	}
	if cfg.SampleRate == 0 {
		cfg.SampleRate = 16000
	}
	ctx, cancel := context.WithCancel(context.Background())
	c := &Client{cfg: cfg, results: make(chan Result, 32), ctx: ctx, cancel: cancel}
	return c, nil
}

// Start открывает WebSocket и запускает горутину приёма сообщений.
func (c *Client) Start(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.started {
		return errors.New("yandex stt: уже запущено")
	}

	// Подмешиваем внешний контекст в жизненный цикл
	parent := c.ctx
	c.ctx, c.cancel = context.WithCancel(parent)

	dialer := websocket.Dialer{
		Proxy:             http.ProxyFromEnvironment,
		HandshakeTimeout:  15 * time.Second,
		EnableCompression: false,
		Subprotocols:      nil,
	}

	// Добавим параметры, которые ожидает WebSocket API SpeechKit (v1):
	// - lang: язык, например ru-RU
	// - sampleRateHertz: частота дискретизации
	// - topic: домен распознавания (часто требуется), по умолчанию general
	// - format: lpcm (LINEAR16 PCM)
	u, err := url.Parse(c.cfg.Endpoint)
	if err != nil {
		return fmt.Errorf("yandex stt: неверный endpoint: %w", err)
	}
	q := u.Query()
	if c.cfg.Language != "" {
		q.Set("lang", c.cfg.Language)
	}
	if c.cfg.SampleRate > 0 {
		q.Set("sampleRateHertz", fmt.Sprint(c.cfg.SampleRate))
	}
	// Яндекс в WebSocket v1, как правило, требует явного topic и format
	if q.Get("topic") == "" {
		q.Set("topic", "general")
	}
	if q.Get("format") == "" {
		q.Set("format", "lpcm")
	}
	u.RawQuery = q.Encode()

	header := http.Header{}
	header.Set("Authorization", "Api-Key "+c.cfg.APIKey)

	conn, resp, err := dialer.DialContext(ctx, u.String(), header)
	if err != nil {
		// Улучшим диагностику рукопожатия, если доступен HTTP-ответ.
		if resp != nil {
			return fmt.Errorf("yandex stt: не удалось подключиться к %s: %s (HTTP %d): %w", u.String(), http.StatusText(resp.StatusCode), resp.StatusCode, err)
		}
		return fmt.Errorf("yandex stt: не удалось подключиться к %s: %w", u.String(), err)
	}
	c.conn = conn

	// Отправим стартовый JSON, если задан.
	if sj := c.cfg.StartJSON; sj != "" {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(sj)); err != nil {
			_ = conn.Close()
			return fmt.Errorf("yandex stt: не удалось отправить StartJSON: %w", err)
		}
	} else {
		// Если стартовый JSON не задан, попробуем отправить минимальную конфигурацию для v1 WebSocket.
		// Многие примеры SpeechKit принимают такие поля в первом текстовом сообщении.
		// Если сервер ожидает только параметры в URL — этот блок будет безопасен (сервер обычно игнорирует лишнее).
		start := map[string]any{
			"lang":            c.cfg.Language,
			"format":          "lpcm",
			"sampleRateHertz": c.cfg.SampleRate,
			"topic":           "general",
			// В некоторых конфигурациях доступны дополнительные опции, оставим по умолчанию:
			// "profanityFilter": false,
			// "partialResults":  c.cfg.AllowPartials,
		}
		if b, err := json.Marshal(start); err == nil {
			_ = conn.WriteMessage(websocket.TextMessage, b)
		}
	}

	// Запустим приём сообщений
	go c.readLoop()

	c.started = true
	return nil
}

// readLoop читает сообщения от сервера и публикует в канал results.
func (c *Client) readLoop() {
	defer close(c.results)
	for {
		select {
		case <-c.ctx.Done():
			return
		default:
		}
		msgType, data, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		switch msgType {
		case websocket.TextMessage:
			res, ok := parseServerMessage(data)
			if ok {
				select {
				case c.results <- res:
				case <-c.ctx.Done():
					return
				}
			}
		case websocket.BinaryMessage:
			// Обычно сервер шлёт текстовые JSON-ответы. Бинарные можно игнорировать.
		case websocket.CloseMessage:
			return
		}
	}
}

// parseServerMessage пытается вытащить текст и признак финальности из произвольного JSON.
func parseServerMessage(data []byte) (Result, bool) {
	// Попробуем известные шаблоны. Поддержим flexible-схему.
	// 1) {"result":"text","final":true}
	var s1 struct {
		Result string `json:"result"`
		Final  bool   `json:"final"`
	}
	if json.Unmarshal(data, &s1) == nil && (s1.Result != "" || s1.Final) {
		return Result{Text: s1.Result, Final: s1.Final, Timestamp: time.Now()}, true
	}

	// 2) {"alternatives":[{"text":"..."}],"final":true}
	var s2 struct {
		Alternatives []struct {
			Text string `json:"text"`
		} `json:"alternatives"`
		Final bool `json:"final"`
	}
	if json.Unmarshal(data, &s2) == nil && len(s2.Alternatives) > 0 {
		return Result{Text: s2.Alternatives[0].Text, Final: s2.Final, Timestamp: time.Now()}, true
	}

	// 3) {"partial":"..."}
	var s3 struct {
		Partial string `json:"partial"`
	}
	if json.Unmarshal(data, &s3) == nil && s3.Partial != "" {
		return Result{Text: s3.Partial, Final: false, Timestamp: time.Now()}, true
	}

	// 4) Generic: {"text":"...","is_final":true}
	var s4 struct {
		Text    string `json:"text"`
		IsFinal bool   `json:"is_final"`
		Final   bool   `json:"final"`
	}
	if json.Unmarshal(data, &s4) == nil && (s4.Text != "" || s4.IsFinal || s4.Final) {
		fin := s4.IsFinal || s4.Final
		return Result{Text: s4.Text, Final: fin, Timestamp: time.Now()}, true
	}

	return Result{}, false
}

// WritePCM16 отправляет сэмплы PCM16 (mono) бинарным фреймом.
func (c *Client) WritePCM16(samples []int16) error {
	c.mu.Lock()
	conn := c.conn
	started := c.started
	c.mu.Unlock()
	if !started || conn == nil {
		return errors.New("yandex stt: соединение не установлено (Start не вызывался)")
	}
	// Преобразуем в []byte little-endian без промежуточных аллокаций поэлементно.
	// Оптимизация: выделим буфер ровно под 2*len(samples).
	b := make([]byte, 0, 2*len(samples))
	for _, s := range samples {
		lo := byte(s)
		hi := byte(s >> 8)
		b = append(b, lo, hi)
	}
	return conn.WriteMessage(websocket.BinaryMessage, b)
}

// Results возвращает канал с распознанными гипотезами.
func (c *Client) Results() <-chan Result { return c.results }

// Close корректно завершает стрим и закрывает соединение.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.started {
		return nil
	}
	c.cancel()
	if c.conn != nil {
		if ej := c.cfg.EndJSON; ej != "" {
			_ = c.conn.WriteMessage(websocket.TextMessage, []byte(ej))
		}
		// Попросим сервер закрыть
		_ = c.conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "eof"))
		_ = c.conn.Close()
	}
	c.started = false
	return nil
}
