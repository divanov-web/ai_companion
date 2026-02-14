package main

import (
	"OpenAIClient/internal/config"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

// Утилита для синтеза речи через Yandex SpeechKit TTS.
// На этом шаге используем открытый API-ключ (можно задать в коде или через переменную окружения YC_TTS_API_KEY).
// Результат сохраняется в файл в текущем каталоге запуска.
func main() {
	// Параметры по умолчанию можно переопределить через флаги CLI
	var (
		text    string
		voice   string
		format  string
		out     string
		play    bool
		emotion string
	)

	cfg := config.NewConfig()

	// Немного разумных дефолтов
	flag.StringVar(&text, "text", "Капитан, мы потерпели поражение, противники в очках взяли верх. Ты командовал \"Mogami\", стрельба из главного калибра была знатной", "Текст для синтеза речи")
	flag.StringVar(&voice, "voice", "ermil", "Голос (например: filipp, jane, oksana, zahar, ermil)")
	flag.StringVar(&emotion, "emotion", "neutral", "Эмоция (например: neutral, good)")
	flag.StringVar(&format, "format", "mp3", "Формат выходного аудио (oggopus|mp3|wav)")
	flag.StringVar(&out, "out", "speech.ogg", "Имя выходного файла (в текущем каталоге)")
	flag.BoolVar(&play, "play", true, "Сразу воспроизвести результат без сохранения файла (поддерживаются wav, mp3)")
	flag.Parse()

	apiKey := cfg.YandexTTS.APIKey
	if env := os.Getenv("YC_TTS_API_KEY"); env != "" {
		apiKey = env
	}

	if apiKey == "" {
		fmt.Println("Ошибка: отсутствует API-ключ. Установите YC_TTS_API_KEY или задайте ключ в коде.")
		os.Exit(1)
	}

	// Подготовим запрос к REST API SpeechKit TTS
	// Документация: https://cloud.yandex.ru/docs/speechkit/tts/request
	endpoint := "https://tts.api.cloud.yandex.net/speech/v1/tts:synthesize"

	data := url.Values{}
	data.Set("text", text)
	data.Set("voice", voice)
	data.Set("format", format)
	data.Set("emotion", emotion)
	// Можно при необходимости настраивать скорость, язык и т.п.:
	// data.Set("speed", "1.0")
	// data.Set("lang", "ru-RU")

	req, err := http.NewRequest(http.MethodPost, endpoint, strings.NewReader(data.Encode()))
	if err != nil {
		fmt.Println("Не удалось создать запрос:", err)
		os.Exit(1)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Api-Key "+apiKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		fmt.Println("Ошибка при выполнении запроса:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// В случае ошибки SpeechKit отдаёт текст с описанием
		b, _ := io.ReadAll(resp.Body)
		fmt.Printf("Сервис вернул ошибку: status=%d, body=%s\n", resp.StatusCode, string(b))
		os.Exit(1)
	}

	// Если запрошено немедленное воспроизведение — играем напрямую из ответа (без файлов)
	if play {
		// Ограничения: oggopus (Opus) напрямую не поддержан в beep.
		// Поддержим wav и mp3. Для oggopus подскажем выбрать другой формат.
		switch strings.ToLower(format) {
		case "wav":
			if err := playWAV(resp.Body); err != nil {
				fmt.Println("Не удалось воспроизвести WAV:", err)
				os.Exit(1)
			}
			fmt.Println("Воспроизведение завершено.")
			return
		case "mp3":
			if err := playMP3(resp.Body); err != nil {
				fmt.Println("Не удалось воспроизвести MP3:", err)
				os.Exit(1)
			}
			fmt.Println("Воспроизведение завершено.")
			return
		default:
			fmt.Println("Формат '", format, "' не поддерживается для прямого воспроизведения. Укажите -format wav или mp3, либо не используйте -play.")
			os.Exit(1)
		}
	}

	// Определим фактическое имя выходного файла в текущем каталоге
	// (чтобы не удивляться temp-пути при go run, используем рабочую директорию)
	wd, err := os.Getwd()
	if err != nil {
		fmt.Println("Не удалось получить текущий каталог:", err)
		os.Exit(1)
	}
	// Если пользователь выбрал mp3/wav — подправим расширение по умолчанию
	outName := out
	switch format {
	case "mp3":
		if filepath.Ext(outName) == "" || filepath.Ext(outName) == ".ogg" {
			outName = "speech.mp3"
		}
	case "wav":
		if filepath.Ext(outName) == "" || filepath.Ext(outName) == ".ogg" {
			outName = "speech.wav"
		}
	default: // oggopus
		if filepath.Ext(outName) == "" {
			outName = "speech.ogg"
		}
	}
	outPath := filepath.Join(wd, outName)

	f, err := os.Create(outPath)
	if err != nil {
		fmt.Println("Не удалось создать файл:", err)
		os.Exit(1)
	}
	defer func() {
		_ = f.Close()
	}()

	if _, err := io.Copy(f, resp.Body); err != nil {
		fmt.Println("Не удалось сохранить аудио:", err)
		os.Exit(1)
	}

	fmt.Printf("Готово. Аудио сохранено в: %s\n", outPath)
}

// Воспроизведение WAV из r (без сохранения в файл)
func playWAV(r io.ReadCloser) error {
	streamer, format, err := wav.Decode(r)
	if err != nil {
		return err
	}
	defer streamer.Close()

	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
		return err
	}
	done := make(chan struct{})
	speaker.Play(beep.Seq(streamer, beep.Callback(func() { close(done) })))
	<-done
	return nil
}

// Воспроизведение MP3 из r (без сохранения в файл)
func playMP3(r io.ReadCloser) error {
	streamer, format, err := mp3.Decode(r)
	if err != nil {
		return err
	}
	defer streamer.Close()

	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
		return err
	}
	done := make(chan struct{})
	speaker.Play(beep.Seq(streamer, beep.Callback(func() { close(done) })))
	<-done
	return nil
}
