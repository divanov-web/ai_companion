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
	logger *zap.SugaredLogger
	path   string
	ply    ttsplayer.Player
}

// NewSoundNotifier создаёт нотификатор. Если path пустой, будет использован
// файл sound/notification1.mp3 рядом с исполняемым файлом.
func NewSoundNotifier(logger *zap.SugaredLogger, path string) *SoundNotifier {
	if strings.TrimSpace(path) == "" {
		// Путь по умолчанию: рядом с бинарём -> sound/notification1.mp3
		if exe, err := os.Executable(); err == nil {
			dir := filepath.Dir(exe)
			cand := filepath.Join(dir, "sound", "notification1.mp3")
			if _, statErr := os.Stat(cand); statErr == nil {
				path = cand
			} else {
				// fallback: от текущей рабочей директории
				path = filepath.Join("sound", "notification1.mp3")
			}
		} else {
			path = filepath.Join("sound", "notification1.mp3")
		}
	}
	return &SoundNotifier{
		logger: logger,
		path:   path,
		ply:    ttsplayer.New(),
	}
}

// Play проигрывает звук уведомления. Ошибки логируются и возвращаются,
// чтобы вызывающий мог принять решение (например, проигнорировать).
func (n *SoundNotifier) Play(ctx context.Context) error {
	// Проверяем отмену контекста до начала
	select {
	case <-ctx.Done():
		return context.Cause(ctx)
	default:
	}

	f, err := os.Open(n.path)
	if err != nil {
		if n.logger != nil {
			n.logger.Warnw("Не удалось открыть звуковой файл уведомления", "path", n.path, "error", err)
		}
		return err
	}

	// Обеспечиваем закрытие файла
	var rc io.ReadCloser = f
	defer rc.Close()

	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(n.path), "."))
	if ext == "" {
		ext = "mp3" // по умолчанию
	}

	if err := n.ply.Play(ext, rc); err != nil {
		if n.logger != nil {
			n.logger.Warnw("Не удалось воспроизвести звуковое уведомление", "path", n.path, "error", err)
		}
		return err
	}
	// Вторичная проверка отмены (если воспроизведение заняло время и контекст отменили)
	if err := context.Cause(ctx); err != nil && !errors.Is(err, nil) {
		return err
	}
	return nil
}
