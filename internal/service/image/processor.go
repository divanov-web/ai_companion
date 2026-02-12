package image

import (
	"bytes"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	defaultMaxWidth     = 1280
	defaultMaxSizeBytes = 1 * 1024 * 1024
	defaultQuality      = 80
)

type ProcessedImage struct {
	Path      string
	Width     int
	Height    int
	SizeBytes int
	MimeType  string
}

type Processor struct {
	outputDir   string
	maxWidth    int
	maxSizeByte int
	quality     int
}

func NewProcessor(outputDir string) *Processor {
	return &Processor{
		outputDir:   outputDir,
		maxWidth:    defaultMaxWidth,
		maxSizeByte: defaultMaxSizeBytes,
		quality:     defaultQuality,
	}
}

func (p *Processor) Process(path string) (ProcessedImage, error) {
	if err := os.MkdirAll(p.outputDir, 0o755); err != nil {
		return ProcessedImage{}, err
	}

	file, err := os.Open(path)
	if err != nil {
		return ProcessedImage{}, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	if err != nil {
		return ProcessedImage{}, err
	}

	origBounds := img.Bounds()
	origWidth := origBounds.Dx()
	origHeight := origBounds.Dy()
	if origWidth == 0 || origHeight == 0 {
		return ProcessedImage{}, fmt.Errorf("invalid image size: %dx%d", origWidth, origHeight)
	}

	quality := max(p.quality, defaultQuality)
	if quality > 100 {
		quality = 100
	}

	resizedWidth := min(origWidth, p.maxWidth)
	resizedHeight := origHeight * resizedWidth / origWidth

	var encoded []byte
	for {
		resized := resizeNearest(img, resizedWidth, resizedHeight)
		encoded, err = encodeJPEG(resized, quality)
		if err != nil {
			return ProcessedImage{}, err
		}

		if len(encoded) <= p.maxSizeByte {
			break
		}

		if resizedWidth <= 320 {
			return ProcessedImage{}, fmt.Errorf("image exceeds max size %d bytes even after downscale", p.maxSizeByte)
		}

		resizedWidth = max(1, int(float64(resizedWidth)*0.9))
		resizedHeight = max(1, origHeight*resizedWidth/origWidth)
	}

	base := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	filename := fmt.Sprintf("%s_%s.jpg", base, time.Now().Format("2006-01-02_15-04-05-000"))
	outputPath := filepath.Join(p.outputDir, filename)
	if err := os.WriteFile(outputPath, encoded, 0o644); err != nil {
		return ProcessedImage{}, err
	}

	return ProcessedImage{
		Path:      outputPath,
		Width:     resizedWidth,
		Height:    resizedHeight,
		SizeBytes: len(encoded),
		MimeType:  "image/jpeg",
	}, nil
}

func encodeJPEG(img image.Image, quality int) ([]byte, error) {
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func resizeNearest(src image.Image, width int, height int) *image.RGBA {
	if width <= 0 || height <= 0 {
		return image.NewRGBA(image.Rect(0, 0, 1, 1))
	}

	srcBounds := src.Bounds()
	srcWidth := srcBounds.Dx()
	srcHeight := srcBounds.Dy()
	if srcWidth == 0 || srcHeight == 0 {
		return image.NewRGBA(image.Rect(0, 0, width, height))
	}

	dst := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		srcY := srcBounds.Min.Y + y*srcHeight/height
		for x := range width {
			srcX := srcBounds.Min.X + x*srcWidth/width
			dst.Set(x, y, src.At(srcX, srcY))
		}
	}

	return dst
}

var _ = png.Decode
