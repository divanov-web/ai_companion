package google

import (
	"OpenAIClient/internal/config"
	"OpenAIClient/internal/service/tts/player"
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"time"

	gctts "cloud.google.com/go/texttospeech/apiv1"
	ttspb "cloud.google.com/go/texttospeech/apiv1/texttospeechpb"
	"go.uber.org/zap"
)

// Client реализует синтез речи через Google Cloud Text-to-Speech и воспроизводит результат.
type Client struct {
	player player.Player
	logger *zap.SugaredLogger
}

func New(p player.Player, logger *zap.SugaredLogger) *Client {
	return &Client{player: p, logger: logger}
}

// Synthesize выполняет запрос к Google TTS и воспроизводит аудио. cfg должен быть config.GoogleTTSConfig.
func (c *Client) Synthesize(ctx context.Context, text string, _ string, cfg any) error {
	gc, ok := cfg.(config.GoogleTTSConfig)
	if !ok {
		return errors.New("google tts: unexpected config type")
	}

	// Создаём клиента SDK
	ttsClient, err := gctts.NewClient(ctx)
	if err != nil {
		return err
	}
	defer ttsClient.Close()

	// Определяем тип входа (text|ssml)
	var input *ttspb.SynthesisInput
	it := strings.ToLower(strings.TrimSpace(gc.InputType))
	if it == "ssml" {
		input = &ttspb.SynthesisInput{InputSource: &ttspb.SynthesisInput_Ssml{Ssml: text}}
	} else {
		input = &ttspb.SynthesisInput{InputSource: &ttspb.SynthesisInput_Text{Text: text}}
	}

	voice := &ttspb.VoiceSelectionParams{
		LanguageCode: gc.Language,
		Name:         gc.Voice, // поддержка Standard/Wavenet голосов
	}

	// Только MP3
	audio := &ttspb.AudioConfig{
		AudioEncoding: ttspb.AudioEncoding_MP3,
		SpeakingRate:  gc.SpeakingRate,
		Pitch:         gc.Pitch,
		VolumeGainDb:  gc.VolumeGainDb,
	}
	if ep := strings.TrimSpace(gc.EffectsProfileID); ep != "" {
		audio.EffectsProfileId = []string{ep}
	}

	req := &ttspb.SynthesizeSpeechRequest{Input: input, Voice: voice, AudioConfig: audio}
	started := time.Now()
	resp, err := ttsClient.SynthesizeSpeech(ctx, req)
	if err != nil {
		return err
	}
	if c.logger != nil {
		c.logger.Infow("Google TTS synthesize completed", "took", time.Since(started).String())
	}

	// Проигрываем MP3
	r := io.NopCloser(bytes.NewReader(resp.GetAudioContent()))
	return c.player.Play("mp3", r)
}
