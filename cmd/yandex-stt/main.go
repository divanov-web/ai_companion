// недоделанная демо-версия клиента stt на yandex platform
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"OpenAIClient/internal/service/stt/yandex"

	"github.com/go-audio/wav"
)

func main() {
	var (
		wavPath    string
		endpoint   string
		lang       string
		chunkMS    int
		sampleRate int
		startJSON  string
		endJSON    string
		printParts bool
		useMic     bool
	)

	flag.StringVar(&wavPath, "wav", "", "путь к WAV файлу (16kHz mono PCM16) для псевдореалтайм-стрима")
	flag.StringVar(&endpoint, "endpoint", "wss://stt.api.cloud.yandex.net/speech/v1/stt:streaming", "WebSocket endpoint Yandex STT")
	flag.StringVar(&lang, "lang", "ru-RU", "язык распознавания (например, ru-RU)")
	flag.IntVar(&chunkMS, "chunk-ms", 50, "длительность чанка в миллисекундах (20-100)")
	flag.IntVar(&sampleRate, "sample-rate", 16000, "частота дискретизации (Гц)")
	flag.StringVar(&startJSON, "start-json", "", "опциональный стартовый JSON, отправляемый текстовым фреймом после подключения")
	flag.StringVar(&endJSON, "end-json", "", "опциональный финальный JSON для сигнала конца аудио")
	flag.BoolVar(&printParts, "print-partials", false, "печатать частичные результаты (если Final недоступен)")
	flag.BoolVar(&useMic, "mic", false, "захватывать звук с микрофона (WASAPI Shared через PortAudio); если задан, -wav игнорируется")
	flag.Parse()

	apiKey := strings.TrimSpace(os.Getenv("YC_STT_API_KEY"))
	if apiKey == "" {
		log.Fatal("YC_STT_API_KEY не задан в окружении")
	}

	cfg := yandex.Config{
		Endpoint:      endpoint,
		APIKey:        apiKey,
		Language:      lang,
		SampleRate:    sampleRate,
		StartJSON:     startJSON,
		EndJSON:       endJSON,
		AllowPartials: printParts,
	}

	client, err := yandex.New(cfg)
	if err != nil {
		log.Fatalf("init stt client: %v", err)
	}

	// Контекст и отмена по Ctrl+C
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := client.Start(ctx); err != nil {
		log.Fatalf("start stt stream: %v", err)
	}
	defer func() { _ = client.Close() }()

	// Чтение результатов в отдельной горутине
	done := make(chan struct{})
	go func() {
		defer close(done)
		for r := range client.Results() {
			if r.Final {
				fmt.Printf("[FINAL] %s\n", r.Text)
			} else if printParts {
				fmt.Printf("[PART] %s\r", r.Text)
			}
		}
	}()

	// Источник аудио: микрофон или WAV-файл в псевдореалтайме.
	if useMic {
		if err := streamMic(ctx, client, sampleRate, chunkMS); err != nil {
			log.Fatalf("stream mic: %v", err)
		}
	} else {
		if wavPath == "" {
			log.Println("не указан -wav и не задан -mic: укажите один из источников аудио")
			<-done
			return
		}
		if err := streamWAV(ctx, client, wavPath, sampleRate, chunkMS); err != nil {
			log.Fatalf("stream wav: %v", err)
		}
	}

	// Дождёмся обработки результатов и корректного закрытия
	<-done
}

func streamWAV(ctx context.Context, client *yandex.Client, path string, expectedSR, chunkMS int) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dec := wav.NewDecoder(f)
	if !dec.IsValidFile() {
		return errors.New("wav: неверный или неподдерживаемый файл")
	}
	// Считаем всю дорожку в память (для простоты), затем будем отдавать чанками времени.
	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return fmt.Errorf("wav decode: %w", err)
	}
	if buf == nil || buf.Format == nil {
		return errors.New("wav: пустой буфер или отсутствует формат")
	}
	if buf.Format.NumChannels != 1 {
		return fmt.Errorf("wav: требуется mono, получено %d канал(ов)", buf.Format.NumChannels)
	}
	if buf.Format.SampleRate != expectedSR {
		return fmt.Errorf("wav: требуется %d Hz, получено %d Hz", expectedSR, buf.Format.SampleRate)
	}
	if buf.SourceBitDepth != 16 {
		return fmt.Errorf("wav: требуется 16-bit PCM, получено %d-bit", buf.SourceBitDepth)
	}

	// Преобразуем к []int16
	// go-audio/audio.IntBuffer хранит данные как []int, нормализованные значением Max*.
	// Здесь buf уже имеет тип *audio.IntBuffer, просто используем его напрямую.
	intBuf := buf
	raw := toInt16(intBuf.Data)

	// Размер чанка по времени
	if chunkMS <= 0 {
		chunkMS = 50
	}
	samplesPerChunk := expectedSR * chunkMS / 1000
	if samplesPerChunk <= 0 {
		samplesPerChunk = 800 // ~50мс при 16кГц
	}

	ticker := time.NewTicker(time.Duration(chunkMS) * time.Millisecond)
	defer ticker.Stop()

	for i := 0; i < len(raw); i += samplesPerChunk {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		end := i + samplesPerChunk
		if end > len(raw) {
			end = len(raw)
		}
		if end > i {
			if err := client.WritePCM16(raw[i:end]); err != nil {
				return err
			}
		}
		// Псевдореалтайм: ждём длительность чанка
		<-ticker.C
	}
	return nil
}

func toInt16(src []int) []int16 {
	// Преобразуем значения из диапазона IntBuffer (обычно -32768..32767 при 16-bit) к int16
	dst := make([]int16, len(src))
	for i, v := range src {
		if v > 32767 {
			v = 32767
		} else if v < -32768 {
			v = -32768
		}
		dst[i] = int16(v)
	}
	return dst
}

// streamMic захватывает звук с микрофона и отправляет в STT чанками по chunkMS.
// Требует PortAudio (DLL в PATH или рядом с бинарём). Формат: mono, int16, sampleRate Гц.
func streamMic(ctx context.Context, client *yandex.Client, sampleRate, chunkMS int) error {
	if chunkMS <= 0 {
		chunkMS = 50
	}
	if sampleRate <= 0 {
		sampleRate = 16000
	}

	// Ленивая инициализация PortAudio
	type porter interface {
		Init() error
		Terminate() error
	}
	// Локальные импорты нельзя в Go, поэтому используем обёртку ниже.
	pa, err := paInit()
	if err != nil {
		return fmt.Errorf("portaudio init: %w", err)
	}
	defer pa.Terminate()

	// Открываем входной поток: mono int16 @ sampleRate
	stream, err := openInputStream(sampleRate)
	if err != nil {
		return fmt.Errorf("open input stream: %w", err)
	}
	defer stream.Close()

	if err := stream.Start(); err != nil {
		return fmt.Errorf("start stream: %w", err)
	}
	defer func() { _ = stream.Stop() }()

	samplesPerChunk := sampleRate * chunkMS / 1000
	if samplesPerChunk <= 0 {
		samplesPerChunk = 800
	}
	buf := make([]int16, samplesPerChunk)

	ticker := time.NewTicker(time.Duration(chunkMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		// Считываем ровно chunk
		if err := stream.Read(buf); err != nil {
			return fmt.Errorf("read mic: %w", err)
		}
		if err := client.WritePCM16(buf); err != nil {
			return err
		}
		<-ticker.C
	}
}

// Ниже — тонкая обёртка над PortAudio, чтобы избежать импорта в верхней части файла.
// Это уменьшает влияние на окружение там, где микрофон не используется.

type paStream interface {
	Start() error
	Stop() error
	Close() error
	Read(samples []int16) error
}

type paHandle interface{ Terminate() error }

func paInit() (paHandle, error)                        { return paInitImpl() }
func openInputStream(sampleRate int) (paStream, error) { return openInputStreamImpl(sampleRate) }
