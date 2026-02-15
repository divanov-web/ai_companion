package image

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Cleaner удаляет старые изображения по TTL в заданной директории.
type Cleaner struct {
	logger *zap.SugaredLogger
}

func NewCleaner(logger *zap.SugaredLogger) *Cleaner { return &Cleaner{logger: logger} }

// Clean удаляет файлы изображений старше ttl из dir. В режиме debug — ничего не делает.
func (c *Cleaner) Clean(dir string, ttl time.Duration, debug bool) {
	if debug {
		c.logger.Infow("DEBUG: очистка старых изображений отключена", "dir", dir, "ttl", ttl.String())
		return
	}
	if ttl <= 0 || dir == "" {
		return
	}

	deadline := time.Now().Add(-ttl)
	exts := []string{".jpg", ".jpeg"}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return
		}
		c.logger.Warnw("Не удалось прочитать директорию для очистки", "dir", dir, "error", err)
		return
	}

	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		lower := strings.ToLower(name)
		if slices.IndexFunc(exts, func(ext string) bool { return strings.HasSuffix(lower, ext) }) == -1 {
			continue
		}
		fi, statErr := e.Info()
		if statErr != nil {
			c.logger.Warnw("Не удалось получить информацию о файле при очистке", "name", name, "error", statErr)
			continue
		}
		if fi.ModTime().Before(deadline) {
			full := filepath.Join(dir, name)
			if err := os.Remove(full); err != nil {
				c.logger.Warnw("Не удалось удалить старый файл", "path", full, "error", err)
				continue
			}
			removed++
		}
	}
	if removed > 0 {
		//c.logger.Infow("Очистка старых изображений выполнена", "dir", dir, "removed", removed, "before", deadline.Format(time.RFC3339))
	}
}
