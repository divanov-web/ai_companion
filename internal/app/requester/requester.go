package requester

import (
	"OpenAIClient/internal/adapter/localconversation"
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/consts"
	"OpenAIClient/internal/service/chat"
	"OpenAIClient/internal/service/companion"
	"OpenAIClient/internal/service/image"
	"OpenAIClient/internal/service/notify"
	"OpenAIClient/internal/service/speech"
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
	cfg             *config.Config
	companion       *companion.Companion
	logger          *zap.SugaredLogger
	localConv       *localconversation.LocalConversation
	speech          *speech.Speech
	chat            *chat.Chat
	notifier        *notify.SoundNotifier
	rnd             *rand.Rand
	requestCount    int // Количество успешных отправок в текущем диалоге
	characterPrompt string
}

func New(cfg *config.Config, companion *companion.Companion, sp *speech.Speech, ch *chat.Chat, notifier *notify.SoundNotifier, logger *zap.SugaredLogger) *Requester {
	r := &Requester{
		cfg:       cfg,
		companion: companion,
		logger:    logger,
		localConv: localconversation.New("", cfg.MaxHistoryRecords),
		speech:    sp,
		chat:      ch,
		notifier:  notifier,
		rnd:       rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	n := len(cfg.CharacterList)
	r.characterPrompt = cfg.CharacterList[r.rnd.Intn(n)]
	return r
}

// SendMessage выполняет сценарий «Послать запрос» один раз.
func (r *Requester) SendMessage(ctx context.Context) (string, error) {
	var userPrompt string

	var b strings.Builder
	b.WriteString(r.cfg.SpeechHeader)

	usedSpeech := false
	if r.speech != nil {
		if msgs := r.speech.Drain(); len(msgs) > 0 {
			for _, m := range msgs {
				b.WriteString("\n- ")
				b.WriteString(m)
			}
			usedSpeech = true
		}
	}

	if !usedSpeech {
		msg := "доложи статус"
		if n := len(r.cfg.SpeechPrompt); n > 0 {
			msg = r.cfg.SpeechPrompt[r.rnd.Intn(n)]
		}
		b.WriteString("\n- ")
		b.WriteString(msg)
	}

	userPrompt = b.String()
	userSpeech := userPrompt

	// Найти последние N картинок
	paths, err := r.pickLastImages(r.cfg.ImagesSourceDir, r.cfg.ImagesToPick)
	if err != nil {
		return "", err
	}
	if len(paths) == 0 {
		r.logger.Infow("Нет доступных изображений для отправки", "dir", r.cfg.ImagesSourceDir)
		return "", nil
	}

	// Подготовить метаданные изображений для отправки (без доп. обработки)
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

	// Ротация характера: каждые N успешных сообщений выбираем новый
	if r.cfg.RotateConversationEach > 0 && r.requestCount >= r.cfg.RotateConversationEach {
		n := len(r.cfg.CharacterList)
		r.characterPrompt = r.cfg.CharacterList[r.rnd.Intn(n)]
		r.requestCount = 0
	}

	// Собрать историю с заголовком из конфига
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

	// Конкатенируем историю с текущим текстом, добавляя разделитель перед историей
	if len(historyWithHeader) > 0 {
		joined := strings.Join(historyWithHeader, "\n")
		userPrompt = "\n" + consts.AISectionSep + "\n" + joined + "\n" + userPrompt
	}

	// ВСТАВИТЬ блок сообщений из чата в самый конец промпта пользователя
	if r.chat != nil {
		if msgs := r.chat.Drain(); len(msgs) > 0 {
			header := r.cfg.ChatHistoryHeader
			if strings.TrimSpace(header) == "" {
				header = "Сообщения из чата"
			}
			// Собираем блок: заголовок + сообщения (каждое с новой строки)
			bChat := strings.Builder{}
			bChat.WriteString("\n")
			bChat.WriteString(consts.AISectionSep)
			bChat.WriteString("\n")
			bChat.WriteString(header)
			for _, m := range msgs {
				bChat.WriteString("\n")
				bChat.WriteString(m)
			}
			// Добавляем в КОНЕЦ уже сформированного userPrompt
			userPrompt = userPrompt + bChat.String()
		}
	}

	// Отправить сообщение с изображениями (stateless)Давно ли мы не были в море?
	r.logger.Infow("Отправка сообщения", "userSpeech", userSpeech, "characterPrompt", r.characterPrompt)
	// Проиграть звук уведомления перед отправкой
	if r.notifier != nil {
		if err := r.notifier.Play(ctx); err != nil {
			r.logger.Debugw("Ошибка проигрывания звука уведомления (пропускаем)", "error", err)
		}
	}
	resp, err := r.companion.SendMessageWithImage(ctx, r.characterPrompt, r.cfg.AssistantPrompt, userPrompt, processed)
	if err != nil {
		return "", err
	}
	r.requestCount++
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
