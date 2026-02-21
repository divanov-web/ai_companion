package screenshotter

import (
	"OpenAIClient/internal/config"
	"context"
	"image"
	"image/draw"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"time"

	"github.com/kbinani/screenshot"
	"go.uber.org/zap"
)

type Screenshotter struct {
	cfg    *config.Config
	logger *zap.SugaredLogger
}

func New(cfg *config.Config, logger *zap.SugaredLogger) *Screenshotter {
	return &Screenshotter{cfg: cfg, logger: logger}
}

// Run запускает бесконечный цикл снятия скриншотов всего экрана.
// Блокирующий метод; обычно запускается в отдельной горутине.
func (s *Screenshotter) Run(ctx context.Context) {
	// Фича-флаг: при выключенном скриншоттере просто выходим
	if s.cfg != nil && !s.cfg.ScreenshotEnabled {
		s.logger.Infow("Screenshotter is disabled by config")
		return
	}
	interval := time.Duration(max(1, s.cfg.ScreenshotIntervalSeconds)) * time.Second
	t := time.NewTicker(interval)
	defer t.Stop()

	// Гарантируем, что директория существует
	if err := os.MkdirAll(s.cfg.ImagesSourceDir, 0o755); err != nil {
		s.logger.Errorw("Failed to create ImagesSourceDir for screenshots", "dir", s.cfg.ImagesSourceDir, "error", err)
		// продолжаем — возможно директорию поправят вручную
	}

	s.logger.Infow("Screenshotter started", "interval", interval.String(), "outputDir", s.cfg.ImagesSourceDir)
	// Немедленно делаем первый кадр
	s.captureOnce()

	for {
		select {
		case <-ctx.Done():
			s.logger.Infow("Screenshotter stopped", "reason", ctx.Err())
			return
		case <-t.C:
			s.captureOnce()
		}
	}
}

func (s *Screenshotter) captureOnce() {
	n := screenshot.NumActiveDisplays()
	if n <= 0 {
		s.logger.Warnw("No active displays detected for screenshot")
		return
	}

	// Вычисляем объединённые границы всех мониторов
	union := image.Rect(0, 0, 0, 0)
	for i := range n {
		b := screenshot.GetDisplayBounds(i)
		if i == 0 {
			union = b
			continue
		}
		union = union.Union(b)
	}

	canvas := image.NewRGBA(union)
	for i := range n {
		b := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureRect(b)
		if err != nil {
			s.logger.Errorw("Failed to capture display", "index", i, "error", err)
			continue
		}
		// Копируем в холст со смещением
		dstPoint := image.Pt(b.Min.X-union.Min.X, b.Min.Y-union.Min.Y)
		dstRect := image.Rectangle{Min: dstPoint, Max: dstPoint.Add(b.Size())}
		draw.Draw(canvas, dstRect, img, image.Point{}, draw.Src)
	}

	// Масштабируем до maxWidth=1280 при необходимости, сохраняя пропорции
	const maxWidth = 1280
	outImg := image.Image(canvas)
	if w := canvas.Bounds().Dx(); w > maxWidth {
		h := canvas.Bounds().Dy()
		scale := float64(maxWidth) / float64(w)
		newW := int(math.Round(float64(w) * scale))
		newH := int(math.Round(float64(h) * scale))
		if newW <= 0 {
			newW = 1
		}
		if newH <= 0 {
			newH = 1
		}
		outImg = resizeNearest(canvas, newW, newH)
	}

	// Сохраняем JPEG с параметрами, согласованными с проектом (quality=90)
	filename := time.Now().Format("2006-01-02_15-04-05-000") + ".jpg"
	fullPath := filepath.Join(s.cfg.ImagesSourceDir, filename)
	file, err := os.Create(fullPath)
	if err != nil {
		s.logger.Errorw("Failed to create screenshot file", "path", fullPath, "error", err)
		return
	}
	defer func() {
		if cerr := file.Close(); cerr != nil {
			s.logger.Errorw("Failed to close screenshot file", "path", fullPath, "error", cerr)
		}
	}()

	if err := jpeg.Encode(file, outImg, &jpeg.Options{Quality: 90}); err != nil {
		s.logger.Errorw("Failed to encode screenshot to JPEG", "path", fullPath, "error", err)
		_ = file.Close()
		_ = os.Remove(fullPath)
		return
	}

	//s.logger.Debugw("Screenshot saved", "path", fullPath, "size", fmt.Sprintf("%dx%d", outImg.Bounds().Dx(), outImg.Bounds().Dy()))
}

// resizeNearest выполняет масштабирование изображения методом ближайшего соседа
func resizeNearest(src image.Image, width int, height int) *image.RGBA {
	if width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}
	srcBounds := src.Bounds()
	srcW := srcBounds.Dx()
	srcH := srcBounds.Dy()
	if srcW == 0 || srcH == 0 {
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}
	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		srcY := srcBounds.Min.Y + y*srcH/height
		for x := range width {
			srcX := srcBounds.Min.X + x*srcW/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}
	return dst
}
