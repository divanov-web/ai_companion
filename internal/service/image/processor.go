package image

// ProcessedImage описывает готовое к отправке изображение.
// Минимально необходимы поля Path и MimeType; остальные могут быть нулями.
type ProcessedImage struct {
	Path      string
	Width     int
	Height    int
	SizeBytes int
	MimeType  string
}
