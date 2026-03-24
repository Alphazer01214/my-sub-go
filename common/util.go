package common

import (
	"fmt"
	"os"
	"time"

	"github.com/go-audio/audio"
	wav "github.com/go-audio/wav"
)

func SRTTimeToString(t time.Duration) string {
	// time -> xx:xx:xx,xxx   h:m:s,ms
	h := int(t.Hours())
	m := int(t.Minutes()) % 60
	s := int(t.Seconds()) % 60
	ms := int(t.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", h, m, s, ms)
}

func convertToMono(buf *audio.IntBuffer) *audio.IntBuffer {
	if buf.Format.NumChannels <= 1 {
		return buf
	}

	channels := buf.Format.NumChannels
	sampleCount := len(buf.Data) / channels

	monoBuf := &audio.IntBuffer{
		Data: make([]int, sampleCount),
		Format: &audio.Format{
			SampleRate:  buf.Format.SampleRate,
			NumChannels: 1,
		},
	}

	for i := 0; i < sampleCount; i++ {
		sum := 0
		for ch := 0; ch < channels; ch++ {
			sum += buf.Data[i*channels+ch]
		}
		monoBuf.Data[i] = sum / channels
	}

	return monoBuf
}

func DecodeWav(path string) ([]float32, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := wav.NewDecoder(f)

	buf, err := dec.FullPCMBuffer()
	if err != nil {
		return nil, err
	} else if dec.SampleRate != 16000 { // 检查采样率
		return nil, fmt.Errorf("unsupported sample rate: %d", dec.SampleRate)
	}

	if dec.NumChans > 1 {
		buf = convertToMono(buf)
	} else if dec.NumChans != 1 { // 检查声道数
		return nil, fmt.Errorf("unsupported number of channels: %d", dec.NumChans)
	}

	samples := buf.AsFloat32Buffer().Data

	return samples, nil
}
