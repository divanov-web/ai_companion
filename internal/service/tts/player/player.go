package player

import (
	"errors"
	"io"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

// Player воспроизводит аудио потоком в зависимости от формата.
type Player interface {
	Play(format string, r io.ReadCloser) error
}

// Default реализует Player и поддерживает mp3 и wav.
type Default struct{ volumeDB float64 }

// New создаёт плеер без изменения громкости (0 dB).
func New() *Default { return &Default{volumeDB: 0} }

// NewWithVolume создаёт плеер с предустановленной громкостью в dB (отрицательные — тише).
func NewWithVolume(db float64) *Default { return &Default{volumeDB: db} }

func (d *Default) Play(format string, r io.ReadCloser) error {
	switch format {
	case "wav", "WAV":
		return playWAV(r, d.volumeDB)
	case "mp3", "MP3":
		return playMP3(r, d.volumeDB)
	default:
		return errors.New("unsupported format for direct playback; use mp3 or wav")
	}
}

func playWAV(r io.ReadCloser, volDB float64) error {
	streamer, format, err := wav.Decode(r)
	if err != nil {
		return err
	}
	defer streamer.Close()

	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
		return err
	}
	vol := &effects.Volume{
		Streamer: streamer,
		Base:     2,
		Volume:   volDB,
		Silent:   false,
	}
	done := make(chan struct{})
	speaker.Play(beep.Seq(vol, beep.Callback(func() { close(done) })))
	<-done
	return nil
}

func playMP3(r io.ReadCloser, volDB float64) error {
	streamer, format, err := mp3.Decode(r)
	if err != nil {
		return err
	}
	defer streamer.Close()

	if err := speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10)); err != nil {
		return err
	}
	vol := &effects.Volume{
		Streamer: streamer,
		Base:     2,
		Volume:   volDB,
		Silent:   false,
	}
	done := make(chan struct{})
	speaker.Play(beep.Seq(vol, beep.Callback(func() { close(done) })))
	<-done
	return nil
}
