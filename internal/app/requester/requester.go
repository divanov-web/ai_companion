package requester

import (
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/companion"
	"OpenAIClient/internal/service/image"
	"cmp"
	"context"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Requester struct {
	cfg            *config.Config
	companion      *companion.Companion
	logger         *zap.SugaredLogger
	conversationID string
	requestCount   int // Количество успешных отправок в текущем диалоге
	rnd            *rand.Rand
}

func New(cfg *config.Config, companion *companion.Companion, logger *zap.SugaredLogger) *Requester {
	return &Requester{
		cfg:       cfg,
		companion: companion,
		logger:    logger,
		rnd:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SendMessage выполняет сценарий «Послать запрос» один раз.
func (r *Requester) SendMessage(ctx context.Context, text string) (string, error) {
	// Ротация диалога: каждые N успешных сообщений создаём новый диалог
	if r.conversationID != "" && r.cfg.RotateConversationEach > 0 && r.requestCount >= r.cfg.RotateConversationEach {
		r.logger.Infow("Ротация диалога по счётчику", "count", r.requestCount, "threshold", r.cfg.RotateConversationEach)
		r.conversationID = ""
		r.requestCount = 0
	}
	// 1. Найти N последних картинок
	paths, err := r.pickLastImages(r.cfg.ImagesSourceDir, r.cfg.ImagesToPick)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		r.logger.Infow("Нет доступных изображений для отправки", "dir", r.cfg.ImagesSourceDir)
		return "", nil
	}

	// 2. Подготовить метаданные изображений для отправки (без дополнительной обработки)
	processed := make([]image.ProcessedImage, 0, len(paths))
	for _, p := range paths {
		processed = append(processed, image.ProcessedImage{
			Path:     p,
			MimeType: "image/jpeg",
		})
	}
	if len(processed) == 0 {
		r.logger.Infow("После обработки не осталось валидных изображений")
		return "", nil
	}

	// 3. Создать диалог при необходимости
	if r.conversationID == "" {
		// Выбрать случайный характер из списка
		characterPrompt := ""
		if n := len(r.cfg.CharacterList); n > 0 {
			characterPrompt = r.cfg.CharacterList[r.rnd.Intn(n)]
		}

		metadata := map[string]string{
			"game":     "Мир кораблей",
			"game_eng": "Mir korabley",
		}
		r.logger.Infow("Запуск диалога", "character", characterPrompt)
		convID, cerr := r.companion.StartConversation(ctx, characterPrompt, r.cfg.StartPrompt, metadata)
		if cerr != nil {
			return "", fmt.Errorf("failed to start conversation: %w", cerr)
		}
		r.conversationID = convID
		r.requestCount = 0
	}

	// 4. Очистка старых изображений перенесена в scheduler (image.Cleaner)

	// 5. Отправить сообщение с изображениями
	r.logger.Infow("Отправка сообщения", "count images", len(processed), "text", text)
	// Передавать пустой systemText: системный текст установлен при создании разговора
	resp, err := r.companion.SendMessageWithImage(ctx, r.conversationID, "", text, processed)
	if err != nil {
		return "", err
	}
	r.requestCount++
	return resp, nil

}

func (r *Requester) pickLastImages(dir string, n int) ([]string, error) {
	if n <= 0 {
		return nil, nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	// Ограничиваем свежесть изображений: не старше TickTimeoutSeconds
	maxAge := time.Duration(r.cfg.TickTimeoutSeconds) * time.Second
	if maxAge <= 0 {
		maxAge = 30 * time.Second
	}
	cutoff := time.Now().Add(-maxAge)

	type fileInfo struct {
		path string
		mod  int64
	}
	files := make([]fileInfo, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if !(strings.HasSuffix(lower, ".jpg") || strings.HasSuffix(lower, ".jpeg")) {
			continue
		}
		fi, statErr := e.Info()
		if statErr != nil {
			r.logger.Warnw("Не удалось получить информацию о файле", "name", name, "error", statErr)
			continue
		}
		// фильтруем по свежести: берём только файлы новее cutoff
		if fi.ModTime().Before(cutoff) {
			continue
		}
		files = append(files, fileInfo{path: filepath.Join(dir, name), mod: fi.ModTime().UnixNano()})
	}

	if len(files) == 0 {
		return nil, nil
	}

	slices.SortFunc(files, func(a, b fileInfo) int { // по убыванию времени
		return -cmp.Compare(a.mod, b.mod)
	})

	if n > len(files) {
		n = len(files)
	}
	out := make([]string, 0, n)
	for i := range n {
		out = append(out, files[i].path)
	}
	return out, nil
}

// cleanupOldImages удаляет файлы изображений старше ttl в указанных директориях.
// Очистка старых изображений вынесена в image.Cleaner и запускается из scheduler
