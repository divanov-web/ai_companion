package main

import (
	"OpenAIClient/internal/config"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// Базовый конверт сообщений VTube Studio Public API
type vtsEnvelope[T any] struct {
	APIName     string `json:"apiName"`
	APIVersion  string `json:"apiVersion"`
	RequestID   string `json:"requestID"`
	MessageType string `json:"messageType"`
	Data        T      `json:"data"`
}

// Запрос аутентификации по уже выданному токену (AuthenticationRequest)
type authReqData struct {
	PluginName          string `json:"pluginName"`
	PluginDeveloper     string `json:"pluginDeveloper"`
	AuthenticationToken string `json:"authenticationToken"`
}

type authRespData struct {
	Authenticated bool   `json:"authenticated"`
	Reason        string `json:"reason"`
}

// Запрос хоткеев в текущей модели
type hotkeysReqData struct{}

type hotkeyInfo struct {
	Name     string `json:"name"`
	HotkeyID string `json:"hotkeyID"`
	// В ответе присутствуют и другие поля, но нам важны имя и идентификатор
}

type hotkeysRespData struct {
	AvailableHotkeys []hotkeyInfo `json:"availableHotkeys"`
}

// Запрос на срабатывание хоткея (эмоции)
type triggerHotkeyReqData struct {
	// Можно указать либо ID, либо имя хоткея. Для простоты используем имя.
	HotkeyName string `json:"hotkeyName,omitempty"`
	HotkeyID   string `json:"hotkeyID,omitempty"`
}

func main() {
	var (
		wsURL = flag.String("ws", "ws://localhost:8001", "WebSocket адрес VTube Studio API")
		name  = flag.String("name", "OpenAIClient VTS Trigger", "Имя плагина (pluginName)")
		dev   = flag.String("dev", "OpenAIClient", "Имя разработчика (pluginDeveloper)")
		id    = flag.String("id", fmt.Sprintf("req-%d", time.Now().UnixNano()), "Произвольный requestID")
		apiV  = flag.String("api-ver", "1.0", "Версия VTS Public API")
	)
	flag.Parse()

	cfg := config.NewConfig()
	if cfg.VTubeAPIKey == "" {
		log.Fatal("переменная окружения VTUBE_API_KEY не задана; получите токен через vtube-auth (старую версию) и пропишите его в окружении")
	}

	// Подключаемся к WebSocket с таймаутом
	ctx, cancel := context.WithTimeoutCause(context.Background(), 10*time.Second, fmt.Errorf("timeout connecting to VTS WS"))
	defer cancel()

	d := websocket.Dialer{HandshakeTimeout: 5 * time.Second}
	conn, _, err := d.DialContext(ctx, *wsURL, nil)
	if err != nil {
		log.Fatalf("не удалось подключиться к %s: %v", *wsURL, err)
	}
	defer func() { _ = conn.Close() }()

	// 1) Аутентификация по токену
	authReq := vtsEnvelope[authReqData]{
		APIName:     "VTubeStudioPublicAPI",
		APIVersion:  *apiV,
		RequestID:   *id,
		MessageType: "AuthenticationRequest",
		Data: authReqData{
			PluginName:          *name,
			PluginDeveloper:     *dev,
			AuthenticationToken: cfg.VTubeAPIKey,
		},
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(authReq); err != nil {
		log.Fatalf("ошибка отправки AuthenticationRequest: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	var raw struct {
		MessageType string          `json:"messageType"`
		Data        json.RawMessage `json:"data"`
	}
	if err := conn.ReadJSON(&raw); err != nil {
		log.Fatalf("ошибка чтения ответа на аутентификацию: %v", err)
	}
	if raw.MessageType != "AuthenticationResponse" {
		b, _ := json.MarshalIndent(raw, "", "  ")
		log.Fatalf("неожиданный ответ на аутентификацию: %s\n%s", raw.MessageType, string(b))
	}
	var aResp authRespData
	if err := json.Unmarshal(raw.Data, &aResp); err != nil {
		log.Fatalf("не удалось распарсить AuthenticationResponse: %v", err)
	}
	if !aResp.Authenticated {
		log.Fatal(errors.New(fmt.Sprintf("аутентификация отклонена: %s", aResp.Reason)))
	}

	// 2) Запрос списка хоткеев (эмоций) текущей модели
	hotReq := vtsEnvelope[hotkeysReqData]{
		APIName:     "VTubeStudioPublicAPI",
		APIVersion:  *apiV,
		RequestID:   fmt.Sprintf("%s-hotkeys", *id),
		MessageType: "HotkeysInCurrentModelRequest",
		Data:        hotkeysReqData{},
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(hotReq); err != nil {
		log.Fatalf("ошибка отправки HotkeysInCurrentModelRequest: %v", err)
	}

	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	raw = struct {
		MessageType string          `json:"messageType"`
		Data        json.RawMessage `json:"data"`
	}{}
	if err := conn.ReadJSON(&raw); err != nil {
		log.Fatalf("ошибка чтения ответа хоткеев: %v", err)
	}
	if raw.MessageType != "HotkeysInCurrentModelResponse" {
		b, _ := json.MarshalIndent(raw, "", "  ")
		log.Fatalf("неожиданный тип ответа на запрос хоткеев: %s\n%s", raw.MessageType, string(b))
	}

	var hResp hotkeysRespData
	if err := json.Unmarshal(raw.Data, &hResp); err != nil {
		log.Fatalf("не удалось распарсить HotkeysInCurrentModelResponse: %v", err)
	}

	if len(hResp.AvailableHotkeys) == 0 {
		fmt.Println("В текущей модели не найдено хоткеев/эмоций")
		return
	}
	fmt.Println("Список эмоций/хоткеев текущей модели:")
	for _, hk := range hResp.AvailableHotkeys {
		if hk.HotkeyID != "" {
			fmt.Printf("- %s (id=%s)\n", hk.Name, hk.HotkeyID)
		} else {
			fmt.Printf("- %s\n", hk.Name)
		}
	}

	// 3) Для теста попробуем вызвать эмоцию/хоткей с именем, содержащим "shock"
	idx := slices.IndexFunc(hResp.AvailableHotkeys, func(h hotkeyInfo) bool {
		ln := strings.ToLower(h.Name)
		return ln == "shock" || strings.Contains(ln, "shock")
	})
	if idx == -1 {
		fmt.Println("Хоткей/эмоция \"shock\" не найдена в текущей модели — пропускаем вызов")
		return
	}

	sel := hResp.AvailableHotkeys[idx]
	display := sel.Name
	if sel.HotkeyID != "" {
		display += fmt.Sprintf(" [id=%s]", sel.HotkeyID)
	}
	fmt.Printf("Пробуем вызвать эмоцию: %s\n", display)

	// В некоторых версиях VTS имя хоткея может не сработать, надёжнее триггерить по ID
	trigReq := vtsEnvelope[triggerHotkeyReqData]{
		APIName:     "VTubeStudioPublicAPI",
		APIVersion:  *apiV,
		RequestID:   fmt.Sprintf("%s-trigger-shock", *id),
		MessageType: "HotkeyTriggerRequest",
		Data: triggerHotkeyReqData{
			HotkeyID:   sel.HotkeyID,
			HotkeyName: sel.Name, // оставляем на случай, если ID пуст и сервер поддерживает поиск по имени
		},
	}
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	if err := conn.WriteJSON(trigReq); err != nil {
		log.Fatalf("ошибка отправки HotkeyTriggerRequest: %v", err)
	}

	// Считаем ответ (необязательно, но полезно для теста)
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	raw = struct {
		MessageType string          `json:"messageType"`
		Data        json.RawMessage `json:"data"`
	}{}
	if err := conn.ReadJSON(&raw); err != nil {
		log.Fatalf("ошибка чтения ответа на HotkeyTriggerRequest: %v", err)
	}
	if raw.MessageType == "HotkeyTriggerResponse" {
		fmt.Println("Эмоция успешно вызвана (HotkeyTriggerResponse)")
	} else {
		b, _ := json.MarshalIndent(raw, "", "  ")
		fmt.Printf("Получен неожиданный ответ после триггера: %s\n%s\n", raw.MessageType, string(b))
	}
}
