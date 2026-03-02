package vtube

import (
	"OpenAIClient/internal/config"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// envelope универсальный конверт VTS Public API
type envelope[T any] struct {
	APIName     string `json:"apiName"`
	APIVersion  string `json:"apiVersion"`
	RequestID   string `json:"requestID"`
	MessageType string `json:"messageType"`
	Data        T      `json:"data"`
}

type authReq struct {
	PluginName          string `json:"pluginName"`
	PluginDeveloper     string `json:"pluginDeveloper"`
	AuthenticationToken string `json:"authenticationToken"`
}

type hotkeysResp struct {
	AvailableHotkeys []struct {
		Name     string `json:"name"`
		HotkeyID string `json:"hotkeyID"`
	} `json:"availableHotkeys"`
}

type triggerReq struct {
	HotkeyName string `json:"hotkeyName,omitempty"`
	HotkeyID   string `json:"hotkeyID,omitempty"`
}

// apiError описывает стандартный ответ ошибки VTS Public API
type apiError struct {
	ErrorID int    `json:"errorID"`
	Message string `json:"message"`
}

// Client — лёгкий клиент VTS: поддерживает подключение, авторизацию, загрузку хоткеев и триггер по имени/ID.
type Client struct {
	cfg   config.VTubeConfig
	token string
	log   *zap.SugaredLogger

	mu     sync.RWMutex
	byName map[string]string // name -> id
	// В он‑деманд режиме постоянного соединения нет: conn/connected не используются.
	// Оставлены для обратной совместимости, но не задействованы.
	conn      *websocket.Conn
	connected bool
}

func New(cfg config.VTubeConfig, token string, log *zap.SugaredLogger) *Client {
	return &Client{cfg: cfg, token: token, log: log, byName: map[string]string{}}
}

// Start выполняет подключение и авто‑переподключение до остановки контекста.
func (c *Client) Start(ctx context.Context) error {
	// Он‑деманд режим: на старте лишь единожды загружаем хоткеи и кэшируем их локально.
	// При неудаче — возвращаем ошибку (основное приложение завершится фатально по требованиям).
	tctx, cancel := context.WithTimeoutCause(ctx, 10*time.Second, errors.New("vtube: initial connect timeout"))
	defer cancel()
	if err := c.loadHotkeys(tctx); err != nil {
		if c.log != nil {
			c.log.Errorw("VTS initial hotkeys load failed", "error", err)
		}
		return err
	}
	if c.log != nil {
		c.log.Infow("VTS client ready (on-demand mode): hotkeys cached")
	}
	// Никаких фоновых горутин не запускаем — подключение будет устанавливаться по запросу.
	return nil
}

// dialAndAuth устанавливает WebSocket‑соединение и проводит аутентификацию.
func (c *Client) dialAndAuth(ctx context.Context) (*websocket.Conn, error) {
	d := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := d.DialContext(ctx, c.cfg.WSURL, nil)
	if err != nil {
		return nil, err
	}

	// Аутентификация
	req := envelope[authReq]{
		APIName:     "VTubeStudioPublicAPI",
		APIVersion:  c.cfg.APIVersion,
		RequestID:   fmt.Sprintf("auth-%d", time.Now().UnixNano()),
		MessageType: "AuthenticationRequest",
		Data: authReq{
			PluginName:          c.cfg.PluginName,
			PluginDeveloper:     c.cfg.PluginDeveloper,
			AuthenticationToken: c.token,
		},
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(req); err != nil {
		_ = conn.Close()
		return nil, err
	}

	var raw struct {
		MessageType string          `json:"messageType"`
		Data        json.RawMessage `json:"data"`
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := conn.ReadJSON(&raw); err != nil {
		_ = conn.Close()
		return nil, err
	}
	if raw.MessageType != "AuthenticationResponse" {
		if raw.MessageType == "APIError" {
			var ae apiError
			if err := json.Unmarshal(raw.Data, &ae); err == nil && (ae.ErrorID != 0 || strings.TrimSpace(ae.Message) != "") {
				_ = conn.Close()
				return nil, fmt.Errorf("vtube auth error: id=%d, message=%s", ae.ErrorID, ae.Message)
			}
		}
		_ = conn.Close()
		return nil, fmt.Errorf("unexpected message: %s", raw.MessageType)
	}
	return conn, nil
}

// loadHotkeys подключается, аутентифицируется, загружает хоткеи и закрывает соединение.
func (c *Client) loadHotkeys(ctx context.Context) error {
	conn, err := c.dialAndAuth(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	// Запрос хоткеев
	hkReq := envelope[struct{}]{
		APIName:     "VTubeStudioPublicAPI",
		APIVersion:  c.cfg.APIVersion,
		RequestID:   fmt.Sprintf("hk-%d", time.Now().UnixNano()),
		MessageType: "HotkeysInCurrentModelRequest",
		Data:        struct{}{},
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(hkReq); err != nil {
		return err
	}
	var raw struct {
		MessageType string          `json:"messageType"`
		Data        json.RawMessage `json:"data"`
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	if err := conn.ReadJSON(&raw); err != nil {
		return err
	}
	if raw.MessageType != "HotkeysInCurrentModelResponse" {
		if raw.MessageType == "APIError" {
			var ae apiError
			if err := json.Unmarshal(raw.Data, &ae); err == nil && (ae.ErrorID != 0 || strings.TrimSpace(ae.Message) != "") {
				return fmt.Errorf("vtube hotkeys error: id=%d, message=%s", ae.ErrorID, ae.Message)
			}
		}
		return fmt.Errorf("unexpected message: %s", raw.MessageType)
	}
	var hkr hotkeysResp
	if err := json.Unmarshal(raw.Data, &hkr); err != nil {
		return err
	}
	m := make(map[string]string, len(hkr.AvailableHotkeys))
	for _, hk := range hkr.AvailableHotkeys {
		name := strings.TrimSpace(hk.Name)
		if name == "" {
			continue
		}
		m[name] = hk.HotkeyID
	}
	c.mu.Lock()
	c.byName = m
	c.mu.Unlock()
	if c.log != nil {
		c.log.Infow("VTS hotkeys loaded", "count", len(m))
	}
	return nil
}

func (c *Client) close() error { // историческая функция; в он‑деманд режиме, как правило, не используется
	c.mu.Lock()
	defer c.mu.Unlock()
	c.connected = false
	if c.conn != nil {
		err := c.conn.Close()
		c.conn = nil
		return err
	}
	return nil
}

// TriggerByNames вызывает список хоткеев по именам, если найдены в карте.
func (c *Client) TriggerByNames(names []string) error {
	// копируем карту хоткеев чтобы уменьшить окно блокировки
	m := map[string]string{}
	for k, v := range c.byName {
		m[k] = v
	}

	// упорядочим имена стабильно
	names = slices.Compact(names)
	// Подготовим список пар name+id к отправке
	type pair struct{ name, id string }
	pairs := make([]pair, 0, len(names))
	for _, n := range names {
		id := m[n]
		if id == "" {
			continue
		}
		pairs = append(pairs, pair{name: n, id: id})
	}
	if len(pairs) == 0 {
		if c.log != nil && len(names) > 0 {
			c.log.Warnw("VTS no matching hotkeys for provided names", "names", names)
		}
		return nil
	}
	if c.log != nil {
		ids := make([]string, 0, len(pairs))
		for _, p := range pairs {
			ids = append(ids, p.id)
		}
	}

	// Он‑деманд подключение: подключаемся, аутентифицируемся, отправляем триггеры и закрываем.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	conn, err := c.dialAndAuth(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()

	for _, p := range pairs {
		req := envelope[triggerReq]{
			APIName:    "VTubeStudioPublicAPI",
			APIVersion: c.cfg.APIVersion,
			RequestID:  fmt.Sprintf("tr-%d", time.Now().UnixNano()),
			// В соответствии с публичным API корректный тип: HotkeyTriggerRequest
			MessageType: "HotkeyTriggerRequest",
			// По запросу: передаём и ID, и имя хоткея (на случай, если серверу требуется имя)
			Data: triggerReq{HotkeyID: p.id, HotkeyName: p.name},
		}
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		if err := conn.WriteJSON(req); err != nil {
			return err
		}
	}
	return nil
}

// TriggerReset вызывает эмоцию сброса, если задано имя ResetEmotion и оно известно среди хоткеев.
func (c *Client) TriggerReset() error {
	name := strings.TrimSpace(c.cfg.ResetEmotion)
	if name == "" {
		return nil
	}
	return c.TriggerByNames([]string{name})
}

// readPump/pingPump больше не используются в он‑деманд режиме; оставлены закомментированными для истории.
