package main

// Реализация обёртки над PortAudio для Windows.
// Требует наличия PortAudio DLL (например, portaudio_x64.dll) в PATH или рядом с бинарём.

import (
	"fmt"

	"github.com/gordonklaus/portaudio"
)

type paHandleImpl struct{}

func paInitImpl() (paHandle, error) {
	if err := portaudio.Initialize(); err != nil {
		return nil, err
	}
	return paHandleImpl{}, nil
}

func (paHandleImpl) Terminate() error { return portaudio.Terminate() }

type paStreamImpl struct {
	stream *portaudio.Stream
	buf    []int16
}

func openInputStreamImpl(sampleRate int) (paStream, error) {
	if sampleRate <= 0 {
		sampleRate = 16000
	}
	// Выберем буфер по умолчанию на 50мс (пересчёт произойдёт в streamMic по фактическому chunk)
	frames := sampleRate / 20 // ~50мс
	if frames < 256 {
		frames = 256
	}
	p := &paStreamImpl{buf: make([]int16, frames)}
	// Открываем 1 входной канал (mono), 0 выходных, int16 буфер
	s, err := portaudio.OpenDefaultStream(1, 0, float64(sampleRate), len(p.buf), &p.buf)
	if err != nil {
		return nil, fmt.Errorf("OpenDefaultStream: %w", err)
	}
	p.stream = s
	return p, nil
}

func (p *paStreamImpl) Start() error { return p.stream.Start() }
func (p *paStreamImpl) Stop() error  { return p.stream.Stop() }
func (p *paStreamImpl) Close() error { return p.stream.Close() }

// Read читает из внутреннего буфера PortAudio и копирует ровно len(dst) сэмплов.
// Если входной буфер короче — читает несколько раз.
func (p *paStreamImpl) Read(dst []int16) error {
	n := len(dst)
	off := 0
	for off < n {
		// Если dst длиннее внутреннего буфера — читаем порциями
		if err := p.stream.Read(); err != nil {
			return err
		}
		// Скопируем доступные сэмплы
		c := copy(dst[off:], p.buf)
		off += c
	}
	return nil
}
