package typedef

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Converter struct {
	Cfg          *Config
	CvtArgs      *ConverterArgs
	isProcessing bool
}

type ConverterArgs struct {
	AudioCodec string `label:"音频编码" type:"string" options:"pcm_s16le"`
	SampleRate int    `label:"采样率" type:"int" placeholder:"16000"`
	Channels   int    `label:"声道数" type:"int" placeholder:"2"`
}

func NewConverter(cfg *Config) *Converter {
	var cvt = &Converter{
		isProcessing: false,
	}
	_ = cvt.Init(cfg)
	return cvt
}

func (cvt *Converter) Init(cfg *Config) error {
	cvt.Cfg = cfg
	cvt.CvtArgs = &ConverterArgs{
		AudioCodec: cvt.Cfg.FFmpeg.AudioCodec,
		SampleRate: cvt.Cfg.FFmpeg.SampleRate,
		Channels:   2,
	}
	return nil
}
func (cvt *Converter) GetWavTmpFile(videoPath string, targetAudioPath string) error {
	cvt.isProcessing = true
	if _, err := os.Stat(targetAudioPath); os.IsNotExist(err) {
		return fmt.Errorf("[FFmpeg] invalid target dir: %v \n %v", targetAudioPath, err)
	}
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("[FFmpeg] invalid video path: %s", videoPath)
	}

	suf := strings.ToLower(filepath.Ext(videoPath))

	for _, s := range VideoType {
		if suf == s {
			args := []string{
				"-i", videoPath,
				"-vn",
				"-acodec", cvt.Cfg.FFmpeg.AudioCodec,
				"-ac", "1",
				"-ar", fmt.Sprintf("%d", cvt.Cfg.FFmpeg.SampleRate),
				"-f", "wav",
				"-y",
				targetAudioPath,
			}
			cmd := exec.Command(cvt.Cfg.FFmpeg.BinaryPath, args...)
			if err := cmd.Run(); err != nil {
				cvt.isProcessing = false
				return fmt.Errorf("[FFmpeg] failed to extract audio: %v \n %v", videoPath, err)
			}
		}
	}
	cvt.isProcessing = false
	return nil
}

func (cvt *Converter) GetVideoWavFile(videoPath string, targetAudioDir string, args *ConverterArgs) error {
	if err := os.MkdirAll(targetAudioDir, 0755); err != nil {
		return fmt.Errorf("[FFmpeg] invalid target dir: %v \n %v", targetAudioDir, err)
	}
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("[FFmpeg] invalid video path: %s", videoPath)
	}

	ext := strings.ToLower(filepath.Ext(videoPath))
	base := strings.TrimSuffix(filepath.Base(videoPath), ext)

	for _, s := range []string{".mp4", ".mkv", ".avi", ".mov", ".wmv", ".flv", ".webm"} {
		if ext == s {
			args := []string{
				"-i", videoPath,
				"-vn",
				"-acodec", args.AudioCodec,
				"-ac", fmt.Sprintf("%d", args.Channels),
				"-ar", fmt.Sprintf("%d", args.SampleRate),
				"-f", "wav",
				"-y",
				filepath.Join(targetAudioDir, base+".wav"),
			}
			cmd := exec.Command(cvt.Cfg.FFmpeg.BinaryPath, args...)
			return cmd.Run()
		}
	}

	return fmt.Errorf("[FFmpeg] unsupported video format: %s", ext)
}

func (cvt *Converter) Update(cfg *Config) {
	cvt.Cfg = cfg
}

func (cvt *Converter) IsProcessing() bool {
	return cvt.isProcessing
}
