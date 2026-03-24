package internal

import (
	"fmt"
	"my-sub-go/common"
	"os/exec"
	"path/filepath"
	"strings"
)

type Converter struct {
	cfg common.FFmpegConfig
}

func (cvt *Converter) Init(cfg common.FFmpegConfig) error {
	cvt.cfg = cfg
	return nil
}
func (cvt *Converter) GetWavTmpFile(videoPath string, targetAudioPath string) error {
	if filepath.Dir(videoPath) == videoPath {
		return fmt.Errorf("[FFmpeg] invalid video path: %s", videoPath)
	}

	suf := strings.ToLower(filepath.Ext(videoPath))

	for _, s := range []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm"} {
		if suf == s {
			args := []string{
				"-i", videoPath,
				"-vn",
				"-acodec", cvt.cfg.AudioCodec,
				"-ac", "1",
				"-ar", fmt.Sprintf("%d", cvt.cfg.SampleRate),
				"-f", "wav",
				"-y",
				targetAudioPath,
			}
			cmd := exec.Command(cvt.cfg.BinaryPath, args...)
			return cmd.Run()
		}
	}
	return fmt.Errorf("[FFmpeg] unsupported video format: %s", suf)
}

func (cvt *Converter) Update(cfg common.FFmpegConfig) {
	cvt.cfg = cfg
}
