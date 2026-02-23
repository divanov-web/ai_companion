package notify

import (
	ttsplayer "OpenAIClient/internal/service/tts/player"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// SoundNotifier инкапсулирует логику проигрывания короткого звука-уведомления.
type SoundNotifier struct {
	logger  *zap.SugaredLogger
	pathAI  string
	pathTTS string
	ply     ttsplayer.Player
}

// NewSoundNotifier создаёт нотификатор. Если путь(и) пустые, будут использованы дефолты:
// AI: sound/notification1.mp3, TTS: sound/notification3.mp3 (сначала пытаемся рядом с бинарём).
func NewSoundNotifier(logger *zap.SugaredLogger, pathAI, pathTTS string) *SoundNotifier {
	resolve := func(def string) string {
		// Путь по умолчанию: рядом с бинарём
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			cand := filepath.Join(dir, def)
			if _, statErr := os.Stat(cand); statErr == nil {
				return cand
			}
		}
		// fallback: от текущей рабочей директории
		return filepath.FromSlash(def)
	}

	if strings.TrimSpace(pathAI) == "" {
		pathAI = resolve(filepath.Join("sound", "notification1.mp3"))
	}
	if strings.TrimSpace(pathTTS) == "" {
		pathTTS = resolve(filepath.Join("sound", "notification3.mp3"))
	}

	return &SoundNotifier{
		logger:  logger,
		pathAI:  pathAI,
		pathTTS: pathTTS,
		ply:     ttsplayer.New(),
	}
}

// Play проигрывает звук уведомления. Ошибки логируются и возвращаются,
// чтобы вызывающий мог принять решение (например, проигнорировать).
func (n *SoundNotifier) play(ctx context.Context, path string) error {
	// Проверяем отмену контекста до начала
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	default:
	}

	f, err := os.Open(path)
	if err != nil {
		if n.logger != nil {
			n.logger.Warnw("Не удалось открыть звуковой файл уведомления", "path", path, "error", err)
		}
		return err
	}

	// Обеспечиваем закрытие файла
	var rc io.ReadCloser = f
	defer rc.Close()

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if ext == "" {
		ext = "mp3" // по умолчанию
	}

	if err := n.ply.Play(ext, rc); err != nil {
		if n.logger != nil {
			n.logger.Warnw("Не удалось воспроизвести звуковое уведомление", "path", path, "error", err)
		}
		return err
	}
	// Вторичная проверка отмены (если воспроизведение заняло время и контекст отменили)
	if err := context.Cause(ctx); err != nil && !errors.Is(err, nil) {
		return err
	}
	return nil
}

// PlayAI проигрывает звук уведомления получения ответа ИИ.
func (n *SoundNotifier) PlayAI(ctx context.Context) error { return n.play(ctx, n.pathAI) }

// PlayTTS проигрывает звук перед началом синтеза речи TTS.
func (n *SoundNotifier) PlayTTS(ctx context.Context) error { return n.play(ctx, n.pathTTS) }
