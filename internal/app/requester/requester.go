package requester

import (
	"OpenAIClient/internal/adapter/localconversation"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/companion"
	"OpenAIClient/internal/service/image"
	"cmp"
	"context"
	"errors"
	"math/rand"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
)

type Requester struct {
	cfg       *config.Config
	companion *companion.Companion
	logger    *zap.SugaredLogger
	localConv *localconversation.LocalConversation
	rnd       *rand.Rand
}

func New(cfg *config.Config, companion *companion.Companion, logger *zap.SugaredLogger) *Requester {
	return &Requester{
		cfg:       cfg,
		companion: companion,
		logger:    logger,
		localConv: localconversation.New("", cfg.MaxHistoryRecords),
		rnd:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
}

// SendMessage выполняет сценарий «Послать запрос» один раз.
func (r *Requester) SendMessage(ctx context.Context, text string) (string, error) {
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

	// 3. Подготовить системный текст: выбираем случайный характер из списка при каждом stateless-вызове
	characterPrompt := ""
	if n := len(r.cfg.CharacterList); n > 0 {
		characterPrompt = r.cfg.CharacterList[r.rnd.Intn(n)]
	}

	// 4. Очистка старых изображений перенесена в scheduler (image.Cleaner)

	// 5. Сформировать историю ответов с заголовком из конфига
	history := r.localConv.History()
	historyWithHeader := history
	if len(history) > 0 {
		header := r.cfg.HistoryHeader
		if strings.TrimSpace(header) == "" {
			header = "история предыдущих ответов AI:"
		}
		historyWithHeader = make([]string, 0, len(history)+1)
		historyWithHeader = append(historyWithHeader, header)
		historyWithHeader = append(historyWithHeader, history...)
	}

	// 6. Отправить сообщение с изображениями (stateless)
	r.logger.Infow("Отправка сообщения", "text", text)
	resp, err := r.companion.SendMessageWithImage(ctx, characterPrompt, r.cfg.StartPrompt, text, historyWithHeader, processed)
	if err != nil {
		return "", err
	}
	// Сохраняем ответ (локальный лимит истории применяется внутри localConv)
	r.localConv.AppendResponse(resp)
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
