package requester

import (
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service"
	"OpenAIClient/internal/service/image"
	"cmp"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"go.uber.org/zap"
)

type Requester struct {
	cfg            *config.Config
	companion      *service.Companion
	processor      *image.Processor
	logger         *zap.SugaredLogger
	conversationID string
}

func New(cfg *config.Config, companion *service.Companion, logger *zap.SugaredLogger) *Requester {
	return &Requester{
		cfg:       cfg,
		companion: companion,
		processor: image.NewProcessor(cfg.ImagesProcessedDir),
		logger:    logger,
	}
}

// RunOnce выполняет сценарий «Послать запрос» один раз.
func (r *Requester) RunOnce(ctx context.Context, text string) (string, error) {
	// 1. Найти N последних картинок
	paths, err := r.pickLastImages(r.cfg.ImagesSourceDir, r.cfg.ImagesToPick)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		r.logger.Infow("Нет доступных изображений для отправки", "dir", r.cfg.ImagesSourceDir)
		return "", nil
	}

	// 2. Обработать картинки
	processed := make([]image.ProcessedImage, 0, len(paths))
	for _, p := range paths {
		img, perr := r.processor.Process(p)
		if perr != nil {
			r.logger.Warnw("Не удалось обработать изображение", "path", p, "error", perr)
			continue
		}
		processed = append(processed, img)
	}
	if len(processed) == 0 {
		r.logger.Infow("После обработки не осталось валидных изображений")
		return "", nil
	}

	// 3. Создать диалог при необходимости
	if r.conversationID == "" {
		startContext := r.cfg.StartPrompt
		metadata := map[string]string{
			"ship":   "Громовержец",
			"battle": "оценка экипажа",
		}
		convID, cerr := r.companion.StartConversation(ctx, startContext, metadata)
		if cerr != nil {
			return "", fmt.Errorf("failed to start conversation: %w", cerr)
		}
		r.conversationID = convID
	}

	// 4. Отправить сообщение с изображениями
	// Перед отправкой выведем текст сообщения
	r.logger.Infow("Отправка..", "text", text)
	return r.companion.SendMessageWithImage(ctx, r.conversationID, text, processed)
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
