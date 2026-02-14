package player

import (
	"errors"
	"io"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

// Player воспроизводит аудио потоком в зависимости от формата.
type Player interface {
	Play(format string, r io.ReadCloser) error
}

// Default реализует Player и поддерживает mp3 и wav.
type Default struct{}

func New() *Default { return &Default{} }

func (d *Default) Play(format string, r io.ReadCloser) error {
	switch format {
	case "wav", "WAV":
		return playWAV(r)
	case "mp3", "MP3":
		return playMP3(r)
	default:
		return errors.New("unsupported format for direct playback; use mp3 or wav")
	}
}

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
