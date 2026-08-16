package common

import (
	"fmt"
	"os"
	"strconv"
	"strings"
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

// SRTTimeParse 解析 "HH:MM:SS,mmm" 或 "HH:MM:SS.mmm"（毫秒 1-3 位均可）。
func SRTTimeParse(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	sep := strings.LastIndexAny(s, ",.")
	if sep < 0 || sep == len(s)-1 {
		return 0, fmt.Errorf("缺少毫秒分隔符: %q", s)
	}
	hms := strings.Split(s[:sep], ":")
	if len(hms) != 3 {
		return 0, fmt.Errorf("时间格式应为 HH:MM:SS,mmm: %q", s)
	}
	h, err1 := strconv.Atoi(hms[0])
	m, err2 := strconv.Atoi(hms[1])
	sec, err3 := strconv.Atoi(hms[2])
	if err1 != nil || err2 != nil || err3 != nil {
		return 0, fmt.Errorf("时间格式应为 HH:MM:SS,mmm: %q", s)
	}
	msStr := s[sep+1:]
	ms, err := strconv.Atoi(msStr)
	if err != nil || ms < 0 {
		return 0, fmt.Errorf("毫秒无效: %q", s)
	}
	for i := len(msStr); i < 3; i++ { // 按实际位数解释
		ms *= 10
	}
	if m > 59 || sec > 59 {
		return 0, fmt.Errorf("时间超出范围: %q", s)
	}
	return time.Duration(h)*time.Hour + time.Duration(m)*time.Minute +
		time.Duration(sec)*time.Second + time.Duration(ms)*time.Millisecond, nil
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
